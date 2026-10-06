// Package httpapi expose l'API REST JSON de Microdash.
package httpapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/arysis/microdash/api/internal/alertes"
	"github.com/arysis/microdash/api/internal/bareme"
	"github.com/arysis/microdash/api/internal/calc"
	"github.com/arysis/microdash/api/internal/chiffre"
	"github.com/arysis/microdash/api/internal/exports"
	"github.com/arysis/microdash/api/internal/store"
)

const (
	cookieSession = "microdash_session"
	dureeSession  = 30 * 24 * time.Hour
	formatDate    = "2006-01-02"
)

type Server struct {
	Store        *store.Store
	Baremes      *bareme.Set
	CookieSecure bool
	Signature    alertes.Signature // liens de désinscription des e-mails
	Chiffre      *chiffre.Cle      // chiffre les clés Stripe ; nil : connexion Stripe désactivée
	StripeURL    string            // vide : API Stripe ; une autre adresse sert aux tests
	echecsStripe sync.Map          // compte → heure du dernier échec de lecture Stripe
	now          func() time.Time
}

func New(st *store.Store, b *bareme.Set, cookieSecure bool, sig alertes.Signature) *Server {
	return &Server{Store: st, Baremes: b, CookieSecure: cookieSecure, Signature: sig, now: time.Now}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", s.health)
		r.Get("/baremes/{annee}", s.getBareme)
		// Lien signé des e-mails : la page l'appelle en JSON, une messagerie en POST de formulaire (RFC 8058).
		r.Post("/alertes/desinscription", s.desinscription)
		r.Group(func(r chi.Router) {
			r.Use(exigerJSON)
			r.Post("/auth/register", s.register)
			r.Post("/auth/login", s.login)
			r.Post("/auth/logout", s.logout)
		})
		r.Group(func(r chi.Router) {
			r.Use(s.authentifier, exigerJSON)
			r.Get("/me", s.me)
			r.Delete("/me", s.deleteMe)
			r.Get("/profil", s.getProfil)
			r.Put("/profil", s.putProfil)
			r.Get("/transactions", s.listTransactions)
			r.Post("/transactions", s.createTransaction)
			r.Put("/transactions/{id}", s.updateTransaction)
			r.Delete("/transactions/{id}", s.deleteTransaction)
			r.Get("/tableau-de-bord", s.tableauDeBord)
			r.Get("/agenda", s.getAgenda)
			r.Get("/exports/saisies-{annee}.csv", s.exportCSV)
			r.Get("/exports/recapitulatif-{annee}.pdf", s.exportPDF)
			r.Put("/agenda/{code}", s.putEcheance)
			r.Get("/alertes/preferences", s.getPreferences)
			r.Put("/alertes/preferences", s.putPreferences)
			r.Get("/depenses-recurrentes", s.listRecurrentes)
			r.Post("/depenses-recurrentes", s.createRecurrente)
			r.Put("/depenses-recurrentes/{id}", s.updateRecurrente)
			r.Delete("/depenses-recurrentes/{id}", s.deleteRecurrente)
			r.Get("/tresorerie", s.getTresorerie)
			r.Put("/tresorerie/solde", s.putSolde)
			r.Get("/stripe", s.getStripe)
			r.Put("/stripe", s.putStripe)
			r.Delete("/stripe", s.deleteStripe)
			r.Post("/stripe/actualiser", s.actualiserStripe)
			r.Put("/stripe/prevision", s.putPrevisionStripe)
		})
	})
	return r
}

// --- Aides ---

type erreurAPI struct {
	Erreur string `json:"erreur"`
}

func ecrireJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func erreur(w http.ResponseWriter, code int, msg string) {
	ecrireJSON(w, code, erreurAPI{Erreur: msg})
}

func erreurInterne(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("erreur interne", "chemin", r.URL.Path, "err", err)
	erreur(w, http.StatusInternalServerError, "erreur interne")
}

func lireJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		erreur(w, http.StatusBadRequest, "JSON invalide : "+err.Error())
		return false
	}
	return true
}

// exigerJSON refuse les écritures qui ne sont pas en JSON : un formulaire d'un autre site
// ne peut pas en envoyer sans requête préalable CORS, ce qui protège des attaques CSRF.
func exigerJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				erreur(w, http.StatusUnsupportedMediaType, "Content-Type application/json requis")
				return
			}
		case http.MethodDelete:
			if r.Header.Get("X-Requested-With") == "" && !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
				erreur(w, http.StatusUnsupportedMediaType, "en-tête X-Requested-With requis")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.Ping(r.Context()); err != nil {
		erreur(w, http.StatusServiceUnavailable, "base indisponible")
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]string{"statut": "ok"})
}

// --- Authentification ---

type cleCtx struct{}

func userID(ctx context.Context) int64 { return ctx.Value(cleCtx{}).(int64) }

func hacherJeton(jeton string) []byte {
	h := sha256.Sum256([]byte(jeton))
	return h[:]
}

func (s *Server) authentifier(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieSession)
		if err != nil {
			erreur(w, http.StatusUnauthorized, "non connecté")
			return
		}
		id, err := s.Store.SessionUser(r.Context(), hacherJeton(c.Value))
		if errors.Is(err, store.ErrNotFound) {
			erreur(w, http.StatusUnauthorized, "session expirée")
			return
		}
		if err != nil {
			erreurInterne(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), cleCtx{}, id)))
	})
}

func (s *Server) ouvrirSession(w http.ResponseWriter, r *http.Request, id int64) bool {
	brut := make([]byte, 32)
	if _, err := rand.Read(brut); err != nil {
		erreurInterne(w, r, err)
		return false
	}
	jeton := base64.RawURLEncoding.EncodeToString(brut)
	expire := s.now().Add(dureeSession)
	if err := s.Store.CreateSession(r.Context(), hacherJeton(jeton), id, expire); err != nil {
		erreurInterne(w, r, err)
		return false
	}
	http.SetCookie(w, &http.Cookie{
		Name: cookieSession, Value: jeton, Path: "/api", Expires: expire,
		HttpOnly: true, Secure: s.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
	return true
}

type identifiants struct {
	Email      string `json:"email"`
	MotDePasse string `json:"mot_de_passe"`
}

func normaliserEmail(e string) (string, bool) {
	e = strings.ToLower(strings.TrimSpace(e))
	a, err := mail.ParseAddress(e)
	return e, err == nil && a.Address == e
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var in identifiants
	if !lireJSON(w, r, &in) {
		return
	}
	email, ok := normaliserEmail(in.Email)
	if !ok {
		erreur(w, http.StatusBadRequest, "adresse e-mail invalide")
		return
	}
	if len(in.MotDePasse) < 10 {
		erreur(w, http.StatusBadRequest, "le mot de passe doit faire au moins 10 caractères")
		return
	}
	hash, err := hashPassword(in.MotDePasse)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	id, err := s.Store.CreateUser(r.Context(), email, hash)
	if errors.Is(err, store.ErrEmailPris) {
		erreur(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	if s.ouvrirSession(w, r, id) {
		ecrireJSON(w, http.StatusCreated, map[string]any{"id": id, "email": email, "profil_complet": false})
	}
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in identifiants
	if !lireJSON(w, r, &in) {
		return
	}
	email, _ := normaliserEmail(in.Email)
	u, err := s.Store.UserByEmail(r.Context(), email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		erreurInterne(w, r, err)
		return
	}
	if u == nil || !checkPassword(in.MotDePasse, u.PasswordHash) {
		erreur(w, http.StatusUnauthorized, "e-mail ou mot de passe incorrect")
		return
	}
	if s.ouvrirSession(w, r, u.ID) {
		s.ecrireMe(w, r, u)
	}
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieSession); err == nil {
		if err := s.Store.DeleteSession(r.Context(), hacherJeton(c.Value)); err != nil {
			erreurInterne(w, r, err)
			return
		}
	}
	http.SetCookie(w, &http.Cookie{Name: cookieSession, Value: "", Path: "/api", MaxAge: -1, HttpOnly: true, Secure: s.CookieSecure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ecrireMe(w http.ResponseWriter, r *http.Request, u *store.User) {
	_, err := s.Store.Profil(r.Context(), u.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]any{"id": u.ID, "email": u.Email, "profil_complet": err == nil})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	u, err := s.Store.UserByID(r.Context(), userID(r.Context()))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	s.ecrireMe(w, r, u)
}

func (s *Server) deleteMe(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteUser(r.Context(), userID(r.Context())); err != nil {
		erreurInterne(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: cookieSession, Value: "", Path: "/api", MaxAge: -1, HttpOnly: true, Secure: s.CookieSecure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

// --- Barème ---

type categorieJSON struct {
	Code string `json:"code"`
	bareme.Categorie
}

func (s *Server) getBareme(w http.ResponseWriter, r *http.Request) {
	annee, err := strconv.Atoi(chi.URLParam(r, "annee"))
	if err != nil {
		erreur(w, http.StatusBadRequest, "année invalide")
		return
	}
	b, exact := s.Baremes.Pour(annee)
	cats := []categorieJSON{}
	for _, code := range []string{"vente_bic", "services_bic", "liberal_ssi", "liberal_cipav"} {
		if c, ok := b.Categories[code]; ok {
			cats = append(cats, categorieJSON{Code: code, Categorie: c})
		}
	}
	for code, c := range b.Categories { // catégories ajoutées dans un fichier de barème
		switch code {
		case "vente_bic", "services_bic", "liberal_ssi", "liberal_cipav":
		default:
			cats = append(cats, categorieJSON{Code: code, Categorie: c})
		}
	}
	ecrireJSON(w, http.StatusOK, map[string]any{
		"annee": b.Annee, "exact": exact, "categories": cats, "cfp": b.CFP,
		"annees_disponibles": s.Baremes.Annees(),
	})
}

// --- Profil ---

type profilJSON struct {
	store.Profil
	DebutActivite string `json:"debut_activite"`
}

func (s *Server) getProfil(w http.ResponseWriter, r *http.Request) {
	p, err := s.Store.Profil(r.Context(), userID(r.Context()))
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "profil non renseigné")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, profilJSON{Profil: *p, DebutActivite: p.DebutActivite.Format(formatDate)})
}

func (s *Server) putProfil(w http.ResponseWriter, r *http.Request) {
	var in profilJSON
	if !lireJSON(w, r, &in) {
		return
	}
	debut, err := time.Parse(formatDate, in.DebutActivite)
	if err != nil {
		erreur(w, http.StatusBadRequest, "date de début d'activité invalide (AAAA-MM-JJ)")
		return
	}
	if in.Periodicite != "mensuelle" && in.Periodicite != "trimestrielle" {
		erreur(w, http.StatusBadRequest, "périodicité : mensuelle ou trimestrielle")
		return
	}
	in.Profil.DebutActivite = debut
	b, _ := s.Baremes.Pour(s.now().Year())
	if err := calc.Valider(in.Profil.VersCalc(), b); err != nil {
		erreur(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.SaveProfil(r.Context(), userID(r.Context()), in.Profil); err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, in)
}

// --- Transactions ---

type transactionJSON struct {
	store.Transaction
	Date string `json:"date"`
}

func versJSON(t store.Transaction) transactionJSON {
	return transactionJSON{Transaction: t, Date: t.Date.Format(formatDate)}
}

func (s *Server) listTransactions(w http.ResponseWriter, r *http.Request) {
	maintenant := s.now()
	du := time.Date(maintenant.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
	au := time.Date(maintenant.Year(), 12, 31, 0, 0, 0, 0, time.UTC)
	var err error
	if v := r.URL.Query().Get("du"); v != "" {
		if du, err = time.Parse(formatDate, v); err != nil {
			erreur(w, http.StatusBadRequest, "paramètre du invalide")
			return
		}
	}
	if v := r.URL.Query().Get("au"); v != "" {
		if au, err = time.Parse(formatDate, v); err != nil {
			erreur(w, http.StatusBadRequest, "paramètre au invalide")
			return
		}
	}
	txs, err := s.Store.Transactions(r.Context(), userID(r.Context()), du, au)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	res := make([]transactionJSON, 0, len(txs))
	for _, t := range txs {
		res = append(res, versJSON(t))
	}
	ecrireJSON(w, http.StatusOK, res)
}

// validerTransaction contrôle une saisie et complète la catégorie d'une recette.
func (s *Server) validerTransaction(w http.ResponseWriter, r *http.Request, in *transactionJSON) bool {
	d, err := time.Parse(formatDate, in.Date)
	if err != nil {
		erreur(w, http.StatusBadRequest, "date invalide (AAAA-MM-JJ)")
		return false
	}
	in.Transaction.Date = d
	if in.Centimes <= 0 {
		erreur(w, http.StatusBadRequest, "le montant doit être positif")
		return false
	}
	in.Libelle = strings.TrimSpace(in.Libelle)
	in.Tiers = strings.TrimSpace(in.Tiers)
	switch in.Type {
	case calc.Depense:
		in.Categorie = ""
		if in.Poste != calc.PosteURSSAF {
			in.Echeance = ""
		} else if !codeURSSAF.MatchString(in.Echeance) {
			erreur(w, http.StatusBadRequest, "choisis la déclaration URSSAF que ce paiement règle")
			return false
		}
	case calc.Recette:
		in.Poste, in.Echeance = "", ""
		p, err := s.Store.Profil(r.Context(), userID(r.Context()))
		if errors.Is(err, store.ErrNotFound) {
			erreur(w, http.StatusConflict, "renseigne ton profil avant de saisir une recette")
			return false
		}
		if err != nil {
			erreurInterne(w, r, err)
			return false
		}
		if in.Categorie == "" {
			in.Categorie = p.Categorie
		}
		if in.Categorie != p.Categorie && in.Categorie != p.CategorieSecondaire {
			erreur(w, http.StatusBadRequest, "catégorie de recette absente du profil")
			return false
		}
	default:
		erreur(w, http.StatusBadRequest, "type : recette ou depense")
		return false
	}
	return true
}

func (s *Server) createTransaction(w http.ResponseWriter, r *http.Request) {
	var in transactionJSON
	if !lireJSON(w, r, &in) || !s.validerTransaction(w, r, &in) {
		return
	}
	t, err := s.Store.CreateTransaction(r.Context(), userID(r.Context()), in.Transaction)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusCreated, versJSON(*t))
}

func idParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		erreur(w, http.StatusBadRequest, "identifiant invalide")
		return 0, false
	}
	return id, true
}

func (s *Server) updateTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	var in transactionJSON
	if !lireJSON(w, r, &in) || !s.validerTransaction(w, r, &in) {
		return
	}
	in.ID = id
	t, err := s.Store.UpdateTransaction(r.Context(), userID(r.Context()), in.Transaction)
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "transaction introuvable")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, versJSON(*t))
}

func (s *Server) deleteTransaction(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	err := s.Store.DeleteTransaction(r.Context(), userID(r.Context()), id)
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "transaction introuvable")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Tableau de bord ---

// anneeParam lit le paramètre annee, l'année en cours par défaut.
func (s *Server) anneeParam(w http.ResponseWriter, r *http.Request) (int, bool) {
	v := r.URL.Query().Get("annee")
	if v == "" {
		return alertes.Aujourdhui(s.now()).Year(), true
	}
	a, err := strconv.Atoi(v)
	if err != nil || a < 2000 || a > 2100 {
		erreur(w, http.StatusBadRequest, "année invalide")
		return 0, false
	}
	return a, true
}

// profilRequis lit le profil, ou répond 409 s'il n'est pas renseigné.
func (s *Server) profilRequis(w http.ResponseWriter, r *http.Request) (*store.Profil, bool) {
	p, err := s.Store.Profil(r.Context(), userID(r.Context()))
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusConflict, "profil non renseigné")
		return nil, false
	}
	if err != nil {
		erreurInterne(w, r, err)
		return nil, false
	}
	return p, true
}

func (s *Server) tableauDeBord(w http.ResponseWriter, r *http.Request) {
	annee, ok := s.anneeParam(w, r)
	if !ok {
		return
	}
	p, ok := s.profilRequis(w, r)
	if !ok {
		return
	}
	uid := userID(r.Context())
	txs, err := s.saisiesCalcul(r.Context(), uid, annee)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	res, err := calc.Calculer(annee, p.VersCalc(), store.VersCalc(txs), s.Baremes)
	if err != nil {
		erreur(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	ecrireJSON(w, http.StatusOK, res)
}

// --- Agenda ---

type echeanceJSON struct {
	calc.Echeance
	Faite   bool   `json:"faite"`
	FaiteLe string `json:"faite_le,omitempty"`
}

func (s *Server) getAgenda(w http.ResponseWriter, r *http.Request) {
	annee, ok := s.anneeParam(w, r)
	if !ok {
		return
	}
	p, ok := s.profilRequis(w, r)
	if !ok {
		return
	}
	uid := userID(r.Context())
	es, err := alertes.Agenda(r.Context(), s.Store, s.Baremes, uid, *p, annee)
	if err != nil {
		erreur(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	faites, err := s.Store.EcheancesFaites(r.Context(), uid)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	res := make([]echeanceJSON, 0, len(es))
	for _, e := range es {
		j := echeanceJSON{Echeance: e}
		if le, ok := faites[e.Code]; ok {
			j.Faite, j.FaiteLe = true, le.In(alertes.Paris).Format(formatDate)
		}
		// Un paiement saisi pour la déclaration vaut « c'est fait ».
		j.Faite = j.Faite || e.Paye > 0
		res = append(res, j)
	}
	ecrireJSON(w, http.StatusOK, map[string]any{
		"annee": annee, "aujourdhui": alertes.Aujourdhui(s.now()).Format(formatDate), "echeances": res,
	})
}

var codeURSSAF = regexp.MustCompile(`^urssaf-\d{4}-(0[1-9]|1[0-2]|t[1-4])$`)

var codeEcheance = regexp.MustCompile(`^(urssaf-\d{4}-(0[1-9]|1[0-2]|t[1-4])|revenus-\d{4}|cfe-\d{4})$`)

func (s *Server) putEcheance(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if !codeEcheance.MatchString(code) {
		erreur(w, http.StatusBadRequest, "code d'échéance invalide")
		return
	}
	var in struct {
		Faite bool `json:"faite"`
	}
	if !lireJSON(w, r, &in) {
		return
	}
	if err := s.Store.MarquerEcheance(r.Context(), userID(r.Context()), code, in.Faite); err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]any{"code": code, "faite": in.Faite})
}

// --- Préférences des rappels ---

func (s *Server) getPreferences(w http.ResponseWriter, r *http.Request) {
	p, err := s.Store.Preferences(r.Context(), userID(r.Context()))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, p)
}

func (s *Server) putPreferences(w http.ResponseWriter, r *http.Request) {
	var in store.Preferences
	if !lireJSON(w, r, &in) {
		return
	}
	if err := s.Store.SavePreferences(r.Context(), userID(r.Context()), in); err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, in)
}

// desinscription coupe tous les rappels d'un compte à partir du jeton signé des e-mails.
func (s *Server) desinscription(w http.ResponseWriter, r *http.Request) {
	jeton := r.URL.Query().Get("jeton")
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var in struct {
			Jeton string `json:"jeton"`
		}
		if !lireJSON(w, r, &in) {
			return
		}
		jeton = in.Jeton
	}
	uid, ok := s.Signature.Verifier(jeton)
	if !ok {
		erreur(w, http.StatusBadRequest, "lien de désinscription invalide")
		return
	}
	err := s.Store.SavePreferences(r.Context(), uid, store.Preferences{})
	if err != nil {
		// Compte supprimé entre-temps : plus rien à couper.
		if _, e := s.Store.UserByID(r.Context(), uid); errors.Is(e, store.ErrNotFound) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Exports ---

func (s *Server) anneeChemin(w http.ResponseWriter, r *http.Request) (int, bool) {
	a, err := strconv.Atoi(chi.URLParam(r, "annee"))
	if err != nil || a < 2000 || a > 2100 {
		erreur(w, http.StatusBadRequest, "année invalide")
		return 0, false
	}
	return a, true
}

func (s *Server) saisiesAnnee(w http.ResponseWriter, r *http.Request, annee int) ([]store.Transaction, bool) {
	txs, err := s.Store.Transactions(r.Context(), userID(r.Context()), time.Date(annee, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(annee, 12, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		erreurInterne(w, r, err)
		return nil, false
	}
	return txs, true
}

// saisiesCalcul renvoie les saisies d'une année et celles de l'année suivante : un paiement URSSAF
// fait en janvier peut régler une période de l'année. Le calcul ne retient de l'année suivante
// que ces paiements.
func (s *Server) saisiesCalcul(ctx context.Context, uid int64, annee int) ([]store.Transaction, error) {
	return s.Store.Transactions(ctx, uid, time.Date(annee, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(annee+1, 12, 31, 0, 0, 0, 0, time.UTC))
}

func telechargement(w http.ResponseWriter, typ, nom string) {
	w.Header().Set("Content-Type", typ)
	w.Header().Set("Content-Disposition", `attachment; filename="`+nom+`"`)
	w.Header().Set("Cache-Control", "no-store")
}

func (s *Server) exportCSV(w http.ResponseWriter, r *http.Request) {
	annee, ok := s.anneeChemin(w, r)
	if !ok {
		return
	}
	txs, ok := s.saisiesAnnee(w, r, annee)
	if !ok {
		return
	}
	b, _ := s.Baremes.Pour(annee)
	telechargement(w, "text/csv; charset=utf-8", fmt.Sprintf("microdash-saisies-%d.csv", annee))
	if err := exports.CSV(w, txs, b); err != nil {
		slog.Error("export CSV", "err", err)
	}
}

func (s *Server) exportPDF(w http.ResponseWriter, r *http.Request) {
	annee, ok := s.anneeChemin(w, r)
	if !ok {
		return
	}
	p, ok := s.profilRequis(w, r)
	if !ok {
		return
	}
	u, err := s.Store.UserByID(r.Context(), userID(r.Context()))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	txs, ok := s.saisiesAnnee(w, r, annee)
	if !ok {
		return
	}
	calcul, err := s.saisiesCalcul(r.Context(), u.ID, annee)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	res, err := calc.Calculer(annee, p.VersCalc(), store.VersCalc(calcul), s.Baremes)
	if err != nil {
		erreur(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	b, _ := s.Baremes.Pour(annee)
	var buf bytes.Buffer
	err = exports.PDF(&buf, exports.Recapitulatif{
		Email: u.Email, Profil: *p, Resultat: res, Saisies: txs, Bareme: b, EditeLe: alertes.Aujourdhui(s.now()),
	})
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	telechargement(w, "application/pdf", fmt.Sprintf("microdash-recapitulatif-%d.pdf", annee))
	_, _ = w.Write(buf.Bytes())
}
