package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/arysis/microdash/api/internal/store"
)

// --- Connecteur Claude : serveur d'autorisation OAuth 2.1 du serveur MCP ---
//
// Claude s'enregistre seul (RFC 7591), envoie la personne sur /autoriser-claude (page de l'app,
// connectée par cookie), puis échange le code contre des jetons avec PKCE (S256). Seules les
// adresses de retour de Claude sont acceptées : un autre site ne peut pas recevoir de code.

const (
	scopeMCP          = "microdash"
	dureeCodeOAuth    = 10 * time.Minute
	dureeJetonAcces   = time.Hour
	dureeJetonRenouv  = 90 * 24 * time.Hour
	pageAutorisation  = "/autoriser-claude"
	prefixeJetonAcces = "mda_"
	prefixeJetonRenou = "mdr_"
)

// hotesRetourOAuth : seuls hôtes acceptés pour redirect_uri (en https).
var hotesRetourOAuth = []string{"claude.ai", "claude.com"}

// retourOAuthAutorise dit si une adresse de retour mène bien chez Claude.
func retourOAuthAutorise(brut string) bool {
	u, err := url.Parse(brut)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Port() != "" {
		return false
	}
	return slices.Contains(hotesRetourOAuth, strings.ToLower(u.Hostname()))
}

// verifierPKCE compare le code_verifier à la code_challenge S256 (RFC 7636).
func verifierPKCE(verifier, challenge string) bool {
	if l := len(verifier); l < 43 || l > 128 {
		return false
	}
	h := sha256.Sum256([]byte(verifier))
	calc := base64.RawURLEncoding.EncodeToString(h[:])
	return subtle.ConstantTimeCompare([]byte(calc), []byte(challenge)) == 1
}

func jetonAleatoire(prefixe string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefixe + base64.RawURLEncoding.EncodeToString(b), nil
}

// baseURL : l'adresse publique du site (SITE_URL), sinon celle de la requête derrière nginx.
func (s *Server) baseURL(r *http.Request) string {
	if s.SiteURL != "" {
		return s.SiteURL
	}
	proto := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		proto = "https"
	}
	return proto + "://" + r.Host
}

func (s *Server) ressourceMCP(r *http.Request) string { return s.baseURL(r) + "/mcp" }

func (s *Server) urlMetadonneesRessource(r *http.Request) string {
	return s.baseURL(r) + "/.well-known/oauth-protected-resource/mcp"
}

func metadonneesJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	ecrireJSON(w, http.StatusOK, v)
}

// metadonneesRessource : RFC 9728, où trouver le serveur d'autorisation du serveur MCP.
func (s *Server) metadonneesRessource(w http.ResponseWriter, r *http.Request) {
	metadonneesJSON(w, map[string]any{
		"resource":                 s.ressourceMCP(r),
		"authorization_servers":    []string{s.baseURL(r)},
		"scopes_supported":         []string{scopeMCP},
		"bearer_methods_supported": []string{"header"},
		"resource_name":            "Microdash",
	})
}

// metadonneesAutorisation : RFC 8414.
func (s *Server) metadonneesAutorisation(w http.ResponseWriter, r *http.Request) {
	b := s.baseURL(r)
	metadonneesJSON(w, map[string]any{
		"issuer":                                     b,
		"authorization_endpoint":                     b + pageAutorisation,
		"token_endpoint":                             b + "/oauth/token",
		"registration_endpoint":                      b + "/oauth/register",
		"revocation_endpoint":                        b + "/oauth/revoke",
		"scopes_supported":                           []string{scopeMCP},
		"response_types_supported":                   []string{"code"},
		"response_modes_supported":                   []string{"query"},
		"grant_types_supported":                      []string{"authorization_code", "refresh_token"},
		"code_challenge_methods_supported":           []string{"S256"},
		"token_endpoint_auth_methods_supported":      []string{"none", "client_secret_post", "client_secret_basic"},
		"revocation_endpoint_auth_methods_supported": []string{"none", "client_secret_post", "client_secret_basic"},
	})
}

// erreurOAuth répond au format des erreurs OAuth (RFC 6749 §5.2).
func erreurOAuth(w http.ResponseWriter, code int, err, description string) {
	w.Header().Set("Cache-Control", "no-store")
	ecrireJSON(w, code, map[string]string{"error": err, "error_description": description})
}

// --- Enregistrement des clients (RFC 7591) ---

type demandeEnregistrement struct {
	RedirectURIs            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	Scope                   string   `json:"scope"`
}

func (s *Server) enregistrerClient(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var in demandeEnregistrement
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		erreurOAuth(w, http.StatusBadRequest, "invalid_client_metadata", "JSON invalide")
		return
	}
	if len(in.RedirectURIs) == 0 || len(in.RedirectURIs) > 5 {
		erreurOAuth(w, http.StatusBadRequest, "invalid_redirect_uri", "une à cinq adresses de retour")
		return
	}
	for _, u := range in.RedirectURIs {
		if !retourOAuthAutorise(u) {
			erreurOAuth(w, http.StatusBadRequest, "invalid_redirect_uri", "seules les adresses de retour de Claude (claude.ai, claude.com) sont acceptées")
			return
		}
	}
	methode := in.TokenEndpointAuthMethod
	switch methode {
	case "":
		methode = "none"
	case "none", "client_secret_post", "client_secret_basic":
	default:
		erreurOAuth(w, http.StatusBadRequest, "invalid_client_metadata", "token_endpoint_auth_method non pris en charge")
		return
	}
	nom := strings.TrimSpace(in.ClientName)
	if nom == "" {
		nom = "Claude"
	}
	if utf8.RuneCountInString(nom) > 100 {
		nom = string([]rune(nom)[:100])
	}
	id, err := jetonAleatoire("mdc_")
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	c := store.ClientOAuth{ID: id, Nom: nom, RedirectURIs: in.RedirectURIs}
	var secret string
	if methode != "none" {
		if secret, err = jetonAleatoire("mds_"); err != nil {
			erreurInterne(w, r, err)
			return
		}
		c.SecretHash = hacherJeton(secret)
	}
	if err := s.Store.CreateClientOAuth(r.Context(), c); err != nil {
		erreurInterne(w, r, err)
		return
	}
	slog.Info("connecteur Claude : client enregistré", "client", nom)
	res := map[string]any{
		"client_id":                  id,
		"client_id_issued_at":        s.now().Unix(),
		"client_name":                nom,
		"redirect_uris":              in.RedirectURIs,
		"token_endpoint_auth_method": methode,
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"scope":                      scopeMCP,
	}
	if secret != "" {
		res["client_secret"] = secret
		res["client_secret_expires_at"] = 0
	}
	w.Header().Set("Cache-Control", "no-store")
	ecrireJSON(w, http.StatusCreated, res)
}

// clientDeLaRequete lit et authentifie le client d'un appel à /oauth/token ou /oauth/revoke.
func (s *Server) clientDeLaRequete(r *http.Request) (*store.ClientOAuth, bool, error) {
	id, secret, basic := r.BasicAuth()
	if basic {
		id, _ = url.QueryUnescape(id)
		secret, _ = url.QueryUnescape(secret)
	} else {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id == "" {
		return nil, false, nil
	}
	c, err := s.Store.ClientOAuth(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if c.SecretHash != nil && subtle.ConstantTimeCompare(hacherJeton(secret), c.SecretHash) != 1 {
		return nil, false, nil
	}
	return c, true, nil
}

// --- Jetons ---

func (s *Server) jetonOAuth(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		erreurOAuth(w, http.StatusBadRequest, "invalid_request", "formulaire invalide")
		return
	}
	c, ok, err := s.clientDeLaRequete(r)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	if !ok {
		erreurOAuth(w, http.StatusUnauthorized, "invalid_client", "client inconnu ou secret invalide")
		return
	}
	ctx := r.Context()
	var accesID int64
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		code, err := s.Store.ConsommerCodeOAuth(ctx, hacherJeton(r.PostForm.Get("code")))
		if errors.Is(err, store.ErrNotFound) {
			erreurOAuth(w, http.StatusBadRequest, "invalid_grant", "code inconnu ou déjà utilisé")
			return
		}
		if err != nil {
			erreurInterne(w, r, err)
			return
		}
		switch {
		case code.ClientID != c.ID:
			erreurOAuth(w, http.StatusBadRequest, "invalid_grant", "code émis pour un autre client")
			return
		case s.now().After(code.Expire):
			erreurOAuth(w, http.StatusBadRequest, "invalid_grant", "code expiré")
			return
		case r.PostForm.Get("redirect_uri") != code.RedirectURI:
			erreurOAuth(w, http.StatusBadRequest, "invalid_grant", "redirect_uri différente")
			return
		case !verifierPKCE(r.PostForm.Get("code_verifier"), code.Challenge):
			erreurOAuth(w, http.StatusBadRequest, "invalid_grant", "code_verifier invalide")
			return
		}
		accesID = code.AccesID
	case "refresh_token":
		h := hacherJeton(r.PostForm.Get("refresh_token"))
		j, err := s.Store.JetonOAuth(ctx, h, store.JetonRenouvellement)
		if errors.Is(err, store.ErrNotFound) {
			erreurOAuth(w, http.StatusBadRequest, "invalid_grant", "jeton de renouvellement inconnu ou expiré")
			return
		}
		if err != nil {
			erreurInterne(w, r, err)
			return
		}
		if j.ClientID != c.ID {
			erreurOAuth(w, http.StatusBadRequest, "invalid_grant", "jeton émis pour un autre client")
			return
		}
		// Le jeton de renouvellement change à chaque usage : l'ancien ne sert plus.
		if supprime, err := s.Store.SupprimerJetonOAuth(ctx, h); err != nil || !supprime {
			if err != nil {
				erreurInterne(w, r, err)
				return
			}
			erreurOAuth(w, http.StatusBadRequest, "invalid_grant", "jeton de renouvellement déjà utilisé")
			return
		}
		accesID = j.AccesID
	default:
		erreurOAuth(w, http.StatusBadRequest, "unsupported_grant_type", "authorization_code ou refresh_token")
		return
	}

	acces, err1 := jetonAleatoire(prefixeJetonAcces)
	renouv, err2 := jetonAleatoire(prefixeJetonRenou)
	if err := errors.Join(err1, err2); err != nil {
		erreurInterne(w, r, err)
		return
	}
	maintenant := s.now()
	if err := s.Store.CreateJetonsOAuth(ctx, accesID, hacherJeton(acces), maintenant.Add(dureeJetonAcces),
		hacherJeton(renouv), maintenant.Add(dureeJetonRenouv)); err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	ecrireJSON(w, http.StatusOK, map[string]any{
		"access_token":  acces,
		"token_type":    "Bearer",
		"expires_in":    int(dureeJetonAcces.Seconds()),
		"refresh_token": renouv,
		"scope":         scopeMCP,
	})
}

// revoquerOAuth (RFC 7009) : un jeton révoqué coupe tout l'accès qu'il porte.
func (s *Server) revoquerOAuth(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		erreurOAuth(w, http.StatusBadRequest, "invalid_request", "formulaire invalide")
		return
	}
	c, ok, err := s.clientDeLaRequete(r)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	if !ok {
		erreurOAuth(w, http.StatusUnauthorized, "invalid_client", "client inconnu ou secret invalide")
		return
	}
	h := hacherJeton(r.PostForm.Get("token"))
	for _, genre := range []string{store.JetonAcces, store.JetonRenouvellement} {
		j, err := s.Store.JetonOAuth(r.Context(), h, genre)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			erreurInterne(w, r, err)
			return
		}
		if j.ClientID == c.ID {
			if err := s.Store.CouperUnAccesOAuth(r.Context(), j.AccesID); err != nil {
				erreurInterne(w, r, err)
				return
			}
		}
	}
	w.WriteHeader(http.StatusOK)
}

// --- Page « Autoriser Claude » (appelée par l'app, connectée par cookie) ---

type demandeAutorisation struct {
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	ResponseType        string `json:"response_type"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	State               string `json:"state"`
	Scope               string `json:"scope"`
	Resource            string `json:"resource"`
	Autoriser           bool   `json:"autoriser"`
}

// controlerDemande vérifie une demande d'autorisation. Le message d'erreur est pour la personne :
// tant que le client et l'adresse de retour ne sont pas sûrs, on ne renvoie jamais vers elle.
func (s *Server) controlerDemande(r *http.Request, d demandeAutorisation) (*store.ClientOAuth, string, error) {
	if d.ClientID == "" {
		return nil, "demande incomplète : client_id manquant", nil
	}
	c, err := s.Store.ClientOAuth(r.Context(), d.ClientID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, "application inconnue : relance la connexion depuis Claude", nil
	}
	if err != nil {
		return nil, "", err
	}
	switch {
	case !slices.Contains(c.RedirectURIs, d.RedirectURI) || !retourOAuthAutorise(d.RedirectURI):
		return nil, "adresse de retour refusée : seule Claude peut se connecter", nil
	case d.ResponseType != "code":
		return nil, "type de réponse non pris en charge", nil
	case d.CodeChallengeMethod != "S256" || len(d.CodeChallenge) < 43 || len(d.CodeChallenge) > 128:
		return nil, "demande non sécurisée : PKCE S256 requis", nil
	case len(d.State) > 1000:
		return nil, "paramètre state trop long", nil
	case d.Resource != "" && strings.TrimSuffix(d.Resource, "/") != s.ressourceMCP(r):
		return nil, "cette autorisation vise un autre serveur que Microdash", nil
	}
	return c, "", nil
}

func lireDemande(q url.Values) demandeAutorisation {
	return demandeAutorisation{
		ClientID: q.Get("client_id"), RedirectURI: q.Get("redirect_uri"), ResponseType: q.Get("response_type"),
		CodeChallenge: q.Get("code_challenge"), CodeChallengeMethod: q.Get("code_challenge_method"),
		State: q.Get("state"), Scope: q.Get("scope"), Resource: q.Get("resource"),
	}
}

// getDemandeOAuth dit à la page qui demande l'accès, ou pourquoi la demande est refusée.
func (s *Server) getDemandeOAuth(w http.ResponseWriter, r *http.Request) {
	c, msg, err := s.controlerDemande(r, lireDemande(r.URL.Query()))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	if msg != "" {
		erreur(w, http.StatusBadRequest, msg)
		return
	}
	u, _ := url.Parse(r.URL.Query().Get("redirect_uri"))
	ecrireJSON(w, http.StatusOK, map[string]string{"client": c.Nom, "retour": u.Hostname()})
}

// autoriserOAuth enregistre le choix de la personne et renvoie l'adresse où retourner chez Claude.
func (s *Server) autoriserOAuth(w http.ResponseWriter, r *http.Request) {
	var d demandeAutorisation
	if !lireJSON(w, r, &d) {
		return
	}
	c, msg, err := s.controlerDemande(r, d)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	if msg != "" {
		erreur(w, http.StatusBadRequest, msg)
		return
	}
	retour, _ := url.Parse(d.RedirectURI)
	q := retour.Query()
	if d.State != "" {
		q.Set("state", d.State)
	}
	q.Set("iss", s.baseURL(r))
	if !d.Autoriser {
		q.Set("error", "access_denied")
		retour.RawQuery = q.Encode()
		ecrireJSON(w, http.StatusOK, map[string]string{"redirection": retour.String()})
		return
	}
	code, err := jetonAleatoire("mdx_")
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	uid := userID(r.Context())
	if err := s.Store.CreateCodeOAuth(r.Context(), uid, c.ID, hacherJeton(code), d.RedirectURI, d.CodeChallenge,
		s.now().Add(dureeCodeOAuth)); err != nil {
		erreurInterne(w, r, err)
		return
	}
	slog.Info("connecteur Claude : accès autorisé", "compte", uid, "client", c.Nom)
	q.Set("code", code)
	retour.RawQuery = q.Encode()
	ecrireJSON(w, http.StatusOK, map[string]string{"redirection": retour.String()})
}

// --- Carte « Claude » de la page Compte ---

func (s *Server) getAccesClaude(w http.ResponseWriter, r *http.Request) {
	acces, err := s.Store.AccesOAuth(r.Context(), userID(r.Context()))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]any{"adresse": s.ressourceMCP(r), "acces": acces})
}

func (s *Server) couperAccesClaude(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.CouperAccesOAuth(r.Context(), userID(r.Context())); err != nil {
		erreurInterne(w, r, err)
		return
	}
	slog.Info("connecteur Claude : accès coupé", "compte", userID(r.Context()))
	w.WriteHeader(http.StatusNoContent)
}
