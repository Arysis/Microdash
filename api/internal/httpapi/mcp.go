package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/arysis/microdash/api/internal/calc"
	"github.com/arysis/microdash/api/internal/store"
)

// --- Connecteur Claude : serveur MCP ---
//
// Chaque outil appelle le même code que l'app (les handlers de l'API), avec le compte du jeton :
// mêmes règles, mêmes messages d'erreur. Rien d'autre n'est exposé (compte, clés, saisies, Qonto, mails).

const instructionsMCP = `Microdash gère l'activité d'un micro-entrepreneur français : tâches, agenda (rendez-vous, échéances URSSAF et impôts) et finances.
Les dates sont au format AAAA-MM-JJ, à l'heure de Paris ; chaque réponse donne « aujourdhui ». « Ce soir » ou « aujourd'hui » = la date du jour.
Avant de modifier, terminer ou supprimer une tâche ou un rendez-vous, retrouve son id avec lister_taches ou agenda.`

// enregistreur garde la réponse d'un handler appelé en interne.
type enregistreur struct {
	code  int
	corps bytes.Buffer
	h     http.Header
}

func (e *enregistreur) Header() http.Header {
	if e.h == nil {
		e.h = http.Header{}
	}
	return e.h
}
func (e *enregistreur) Write(b []byte) (int, error) {
	if e.code == 0 {
		e.code = http.StatusOK
	}
	return e.corps.Write(b)
}
func (e *enregistreur) WriteHeader(code int) {
	if e.code == 0 {
		e.code = code
	}
}

// appel décrit un appel interne à un handler de l'API.
type appel struct {
	methode string
	params  map[string]string // paramètres d'adresse ({id}, {code})
	query   url.Values
	corps   any
}

// appeler exécute le handler comme si l'app l'appelait pour ce compte, et décode sa réponse JSON
// dans dest (si non nil). Une réponse d'erreur devient une erreur avec le message de l'API.
func (s *Server) appeler(ctx context.Context, uid int64, h http.HandlerFunc, a appel, dest any) error {
	var corps bytes.Buffer
	if a.corps != nil {
		if err := json.NewEncoder(&corps).Encode(a.corps); err != nil {
			return err
		}
	}
	rctx := chi.NewRouteContext()
	for k, v := range a.params {
		rctx.URLParams.Add(k, v)
	}
	ctx = context.WithValue(context.WithValue(ctx, cleCtx{}, uid), chi.RouteCtxKey, rctx)
	req, err := http.NewRequestWithContext(ctx, a.methode, "/api/mcp?"+a.query.Encode(), &corps)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	w := &enregistreur{}
	h(w, req)
	if w.code == 0 {
		w.code = http.StatusOK
	}
	if w.code >= 400 {
		var e erreurAPI
		if json.Unmarshal(w.corps.Bytes(), &e) == nil && e.Erreur != "" {
			if w.code == http.StatusConflict && e.Erreur == "profil non renseigné" {
				return errors.New("profil non renseigné : remplis « Ton activité » dans Microdash d'abord")
			}
			return errors.New(e.Erreur)
		}
		return fmt.Errorf("erreur %d", w.code)
	}
	if dest != nil && w.corps.Len() > 0 {
		return json.Unmarshal(w.corps.Bytes(), dest)
	}
	return nil
}

// compteMCP lit le compte porté par le jeton de l'appel.
func compteMCP(req *mcp.CallToolRequest) (int64, error) {
	if req.Extra == nil || req.Extra.TokenInfo == nil {
		return 0, errors.New("non connecté")
	}
	uid, ok := req.Extra.TokenInfo.Extra["uid"].(int64)
	if !ok {
		return 0, errors.New("non connecté")
	}
	return uid, nil
}

// outil ajoute un outil dont le handler reçoit le compte et renvoie une valeur à envoyer en JSON.
func outil[In any](s *Server, srv *mcp.Server, t *mcp.Tool, f func(ctx context.Context, uid int64, in In) (any, error)) {
	mcp.AddTool(srv, t, func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
		uid, err := compteMCP(req)
		if err != nil {
			return nil, nil, err
		}
		debut := time.Now()
		res, err := f(ctx, uid, in)
		// Journal sans contenu : quel outil, pour quel compte, réussi ou non.
		slog.Info("connecteur Claude : outil", "outil", t.Name, "compte", uid, "ok", err == nil, "duree_ms", time.Since(debut).Milliseconds())
		if err != nil {
			return nil, nil, err
		}
		texte, err := json.Marshal(res)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(texte)}}}, nil, nil
	})
}

func lecture(titre string) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: titre, ReadOnlyHint: true}
}

func ecriture(titre string, destructif bool) *mcp.ToolAnnotations {
	return &mcp.ToolAnnotations{Title: titre, DestructiveHint: &destructif, OpenWorldHint: new(bool)}
}

// --- Entrées des outils ---

type entreeVide struct{}

type entreeListerTaches struct {
	Projet string `json:"projet,omitempty" jsonschema:"filtre facultatif : educari, saas, microdash ou perso"`
}

type entreeTache struct {
	Titre    string `json:"titre" jsonschema:"la tâche, 200 caractères au plus (ex. : Relancer Paul)"`
	Notes    string `json:"notes,omitempty" jsonschema:"détails facultatifs"`
	Projet   string `json:"projet,omitempty" jsonschema:"facultatif : educari, saas, microdash ou perso"`
	Duree    int    `json:"duree_minutes,omitempty" jsonschema:"durée estimée en minutes, facultative"`
	Quand    string `json:"quand,omitempty" jsonschema:"dates (un soir précis ou du debut au fin ; pour ce soir : dates sans debut), semaine (la semaine qui contient debut, sinon la semaine en cours) ou plus_tard (sans date). Par défaut : semaine"`
	Debut    string `json:"debut,omitempty" jsonschema:"AAAA-MM-JJ : premier soir où la faire ; vide = aujourd'hui"`
	Fin      string `json:"fin,omitempty" jsonschema:"AAAA-MM-JJ : dernier soir, pour quand=dates ; vide = le même jour que debut"`
	Echeance string `json:"echeance,omitempty" jsonschema:"AAAA-MM-JJ : date limite facultative"`
}

type entreeModifierTache struct {
	ID       int64   `json:"id" jsonschema:"id de la tâche (lister_taches)"`
	Titre    *string `json:"titre,omitempty" jsonschema:"nouveau titre"`
	Notes    *string `json:"notes,omitempty" jsonschema:"nouvelles notes (vide pour effacer)"`
	Projet   *string `json:"projet,omitempty" jsonschema:"educari, saas, microdash, perso, ou vide pour aucun"`
	Duree    *int    `json:"duree_minutes,omitempty" jsonschema:"durée en minutes (0 pour effacer)"`
	Quand    *string `json:"quand,omitempty" jsonschema:"dates, semaine ou plus_tard ; avec debut et fin comme pour ajouter_tache"`
	Debut    *string `json:"debut,omitempty" jsonschema:"AAAA-MM-JJ"`
	Fin      *string `json:"fin,omitempty" jsonschema:"AAAA-MM-JJ"`
	Echeance *string `json:"echeance,omitempty" jsonschema:"AAAA-MM-JJ, ou vide pour effacer"`
}

type entreeID struct {
	ID int64 `json:"id" jsonschema:"id de l'élément"`
}

type entreeAgenda struct {
	Du string `json:"du" jsonschema:"AAAA-MM-JJ, premier jour"`
	Au string `json:"au" jsonschema:"AAAA-MM-JJ, dernier jour inclus (100 jours au plus après du)"`
}

type entreeRendezVous struct {
	Titre string `json:"titre" jsonschema:"titre du rendez-vous"`
	Jour  string `json:"jour" jsonschema:"AAAA-MM-JJ"`
	Heure string `json:"heure,omitempty" jsonschema:"HH:MM à l'heure de Paris ; vide = toute la journée"`
	Duree int    `json:"duree_minutes,omitempty" jsonschema:"durée en minutes, avec une heure seulement"`
	Notes string `json:"notes,omitempty" jsonschema:"détails facultatifs"`
}

type entreeEcheance struct {
	Code  string `json:"code" jsonschema:"code de l'échéance, le champ ref d'un élément de type echeance dans agenda (ex. : urssaf-2026-10)"`
	Faite bool   `json:"faite" jsonschema:"true : déclarée ou payée ; false : à faire de nouveau"`
}

type entreeFinances struct {
	Annee int `json:"annee,omitempty" jsonschema:"année civile ; par défaut l'année en cours"`
}

// --- Le serveur MCP ---

func (s *Server) serveurMCP() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "microdash", Title: "Microdash", Version: "1.0.0"},
		&mcp.ServerOptions{Instructions: instructionsMCP})

	outil(s, srv, &mcp.Tool{Name: "ce_soir", Annotations: lecture("Ce soir"),
		Description: "La liste « Ce soir » de Microdash : tâches à faire ce soir, échéances proches et rappels des modules actifs."},
		func(ctx context.Context, uid int64, _ entreeVide) (any, error) {
			var res map[string]any
			err := s.appeler(ctx, uid, s.ceSoir, appel{methode: http.MethodGet}, &res)
			return res, err
		})

	outil(s, srv, &mcp.Tool{Name: "lister_taches", Annotations: lecture("Lister les tâches"),
		Description: "Les tâches à faire (et celles cochées aujourd'hui), avec leur id, leur période (debut, fin) et leur projet."},
		func(ctx context.Context, uid int64, in entreeListerTaches) (any, error) {
			var ts []tacheJSON
			if err := s.appeler(ctx, uid, s.listTaches, appel{methode: http.MethodGet}, &ts); err != nil {
				return nil, err
			}
			res := []tacheJSON{}
			for _, t := range ts {
				if in.Projet == "" || t.Projet == in.Projet {
					res = append(res, t)
				}
			}
			return map[string]any{"aujourdhui": s.aujourdhui().Format(formatDate), "taches": res}, nil
		})

	outil(s, srv, &mcp.Tool{Name: "ajouter_tache", Annotations: ecriture("Ajouter une tâche", false),
		Description: "Ajoute une tâche dans Microdash. Pour « ce soir » : quand=dates sans debut. Pour un jour précis : quand=dates et debut=ce jour. Pour cette semaine : quand=semaine. Sans date : quand=plus_tard."},
		func(ctx context.Context, uid int64, in entreeTache) (any, error) {
			t := tacheJSON{Tache: store.Tache{Titre: in.Titre, Notes: in.Notes, Projet: in.Projet, Duree: in.Duree,
				Quand: in.Quand, Debut: in.Debut, Fin: in.Fin, Echeance: in.Echeance}}
			var res tacheJSON
			err := s.appeler(ctx, uid, s.createTache, appel{methode: http.MethodPost, corps: t}, &res)
			return res, err
		})

	outil(s, srv, &mcp.Tool{Name: "modifier_tache", Annotations: ecriture("Modifier une tâche", false),
		Description: "Modifie une tâche : seuls les champs donnés changent. Pour changer de jour, donne quand=dates et debut (et fin)."},
		func(ctx context.Context, uid int64, in entreeModifierTache) (any, error) {
			t, err := s.tacheMCP(ctx, uid, in.ID)
			if err != nil {
				return nil, err
			}
			remplacer(&t.Titre, in.Titre)
			remplacer(&t.Notes, in.Notes)
			remplacer(&t.Projet, in.Projet)
			remplacer(&t.Duree, in.Duree)
			remplacer(&t.Echeance, in.Echeance)
			if in.Quand != nil || in.Debut != nil || in.Fin != nil {
				remplacer(&t.Quand, in.Quand)
				t.Debut, t.Fin = "", ""
				remplacer(&t.Debut, in.Debut)
				remplacer(&t.Fin, in.Fin)
			}
			var res tacheJSON
			err = s.appeler(ctx, uid, s.updateTache, appel{methode: http.MethodPut, params: idParams(in.ID), corps: t}, &res)
			return res, err
		})

	outil(s, srv, &mcp.Tool{Name: "terminer_tache", Annotations: ecriture("Cocher une tâche", false),
		Description: "Coche une tâche comme faite."},
		func(ctx context.Context, uid int64, in entreeID) (any, error) {
			t, err := s.tacheMCP(ctx, uid, in.ID)
			if err != nil {
				return nil, err
			}
			t.Faite = true
			var res tacheJSON
			err = s.appeler(ctx, uid, s.updateTache, appel{methode: http.MethodPut, params: idParams(in.ID), corps: t}, &res)
			return res, err
		})

	outil(s, srv, &mcp.Tool{Name: "supprimer_tache", Annotations: ecriture("Supprimer une tâche", true),
		Description: "Supprime définitivement une tâche. Demande confirmation à la personne avant."},
		func(ctx context.Context, uid int64, in entreeID) (any, error) {
			err := s.appeler(ctx, uid, s.deleteTache, appel{methode: http.MethodDelete, params: idParams(in.ID)}, nil)
			return map[string]any{"supprimee": err == nil, "id": in.ID}, err
		})

	outil(s, srv, &mcp.Tool{Name: "agenda", Annotations: lecture("Lire l'agenda"),
		Description: "L'agenda entre deux dates : rendez-vous (type evenement, ref = id du rendez-vous), tâches datées (type tache) et échéances URSSAF et impôts (type echeance, ref = code, fait = déclarée ou payée)."},
		func(ctx context.Context, uid int64, in entreeAgenda) (any, error) {
			var res map[string]any
			err := s.appeler(ctx, uid, s.getCalendrier, appel{methode: http.MethodGet,
				query: url.Values{"du": {in.Du}, "au": {in.Au}}}, &res)
			return res, err
		})

	outil(s, srv, &mcp.Tool{Name: "ajouter_rendez_vous", Annotations: ecriture("Ajouter un rendez-vous", false),
		Description: "Ajoute un rendez-vous dans l'agenda de Microdash, à une heure précise ou pour toute la journée."},
		func(ctx context.Context, uid int64, in entreeRendezVous) (any, error) {
			e := store.Evenement{Titre: in.Titre, Jour: in.Jour, Heure: in.Heure, Duree: in.Duree, Notes: in.Notes}
			var res store.Evenement
			err := s.appeler(ctx, uid, s.createEvenement, appel{methode: http.MethodPost, corps: e}, &res)
			return res, err
		})

	outil(s, srv, &mcp.Tool{Name: "supprimer_rendez_vous", Annotations: ecriture("Supprimer un rendez-vous", true),
		Description: "Supprime définitivement un rendez-vous (id = ref dans agenda). Demande confirmation à la personne avant."},
		func(ctx context.Context, uid int64, in entreeID) (any, error) {
			err := s.appeler(ctx, uid, s.deleteEvenement, appel{methode: http.MethodDelete, params: idParams(in.ID)}, nil)
			return map[string]any{"supprime": err == nil, "id": in.ID}, err
		})

	outil(s, srv, &mcp.Tool{Name: "marquer_echeance", Annotations: ecriture("Marquer une échéance", false),
		Description: "Marque une échéance URSSAF ou d'impôt comme faite (déclarée ou payée), ou de nouveau à faire."},
		func(ctx context.Context, uid int64, in entreeEcheance) (any, error) {
			var res map[string]any
			err := s.appeler(ctx, uid, s.putEcheance, appel{methode: http.MethodPut,
				params: map[string]string{"code": in.Code}, corps: map[string]bool{"faite": in.Faite}}, &res)
			return res, err
		})

	outil(s, srv, &mcp.Tool{Name: "resume_finances", Annotations: lecture("Résumé des finances"),
		Description: "Résumé de l'année : chiffre d'affaires, cotisations URSSAF estimées, dépenses, revenu net, seuils (plafonds de CA) et solde de trésorerie. Montants en euros."},
		func(ctx context.Context, uid int64, in entreeFinances) (any, error) {
			return s.resumeFinances(ctx, uid, in.Annee)
		})

	return srv
}

func remplacer[T any](champ *T, v *T) {
	if v != nil {
		*champ = *v
	}
}

func idParams(id int64) map[string]string {
	return map[string]string{"id": strconv.FormatInt(id, 10)}
}

// tacheMCP lit une tâche du compte, au format que l'API attend en écriture.
func (s *Server) tacheMCP(ctx context.Context, uid, id int64) (tacheJSON, error) {
	t, err := s.Store.Tache(ctx, uid, id)
	if errors.Is(err, store.ErrNotFound) {
		return tacheJSON{}, errors.New("tâche introuvable")
	}
	if err != nil {
		return tacheJSON{}, err
	}
	return versTacheJSON(*t), nil
}

// euros convertit des centimes en euros pour la lecture par Claude.
func euros(c int64) float64 { return float64(c) / 100 }

type totauxEuros struct {
	CA          float64 `json:"chiffre_affaires"`
	Cotisations float64 `json:"cotisations_estimees"`
	CFP         float64 `json:"cfp"`
	ImpotVL     float64 `json:"impot_versement_liberatoire"`
	URSSAFPaye  float64 `json:"urssaf_deja_paye"`
	Depenses    float64 `json:"depenses"`
	Net         float64 `json:"revenu_net"`
}

func versEuros(t calc.Totaux) totauxEuros {
	return totauxEuros{euros(t.CA), euros(t.Cotisations), euros(t.CFP), euros(t.ImpotVL), euros(t.URSSAFPaye), euros(t.Depenses), euros(t.Net)}
}

func (s *Server) resumeFinances(ctx context.Context, uid int64, annee int) (any, error) {
	q := url.Values{}
	if annee != 0 {
		q.Set("annee", strconv.Itoa(annee))
	}
	var r calc.Resultat
	if err := s.appeler(ctx, uid, s.tableauDeBord, appel{methode: http.MethodGet, query: q}, &r); err != nil {
		return nil, err
	}
	auj := s.aujourdhui().Format(formatDate)
	type moisEuros struct {
		Mois string `json:"mois"`
		totauxEuros
	}
	mois := []moisEuros{}
	for _, m := range r.Mois {
		if m.Debut <= auj {
			mois = append(mois, moisEuros{m.Libelle, versEuros(m.Totaux)})
		}
	}
	type seuil struct {
		Libelle string  `json:"libelle"`
		CA      float64 `json:"chiffre_affaires"`
		Plafond float64 `json:"plafond"`
		Part    string  `json:"part_atteinte"`
		Niveau  string  `json:"niveau"`
	}
	seuils := []seuil{}
	for _, p := range r.Plafonds {
		seuils = append(seuils, seuil{p.Libelle, euros(p.CA), euros(p.Plafond), fmt.Sprintf("%.0f %%", p.Ratio*100), p.Niveau})
	}
	res := map[string]any{
		"aujourdhui": auj, "annee": r.Annee, "total": versEuros(r.Total), "mois": mois, "seuils": seuils,
	}
	if !r.BaremeExact {
		res["note"] = fmt.Sprintf("taux du barème %d utilisés : ceux de %d ne sont pas encore connus", r.BaremeAnnee, r.Annee)
	}
	if so, err := s.Store.Solde(ctx, uid); err != nil {
		return nil, err
	} else if so != nil {
		res["solde_tresorerie"] = map[string]any{"euros": euros(so.Centimes), "au": so.Au}
	}
	return res, nil
}

// --- Branchement HTTP ---

// handlerMCP sert /mcp : jeton OAuth obligatoire, puis le transport HTTP de MCP sans session.
func (s *Server) handlerMCP() http.Handler {
	srv := s.serveurMCP()
	transport := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	verifier := func(ctx context.Context, jeton string, _ *http.Request) (*auth.TokenInfo, error) {
		if !strings.HasPrefix(jeton, prefixeJetonAcces) {
			return nil, auth.ErrInvalidToken
		}
		j, err := s.Store.JetonOAuth(ctx, hacherJeton(jeton), store.JetonAcces)
		if errors.Is(err, store.ErrNotFound) {
			return nil, auth.ErrInvalidToken
		}
		if err != nil {
			return nil, err
		}
		return &auth.TokenInfo{Scopes: []string{scopeMCP}, Expiration: j.Expire,
			UserID: strconv.FormatInt(j.UserID, 10), Extra: map[string]any{"uid": j.UserID}}, nil
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth.RequireBearerToken(verifier, &auth.RequireBearerTokenOptions{
			ResourceMetadataURL: s.urlMetadonneesRessource(r), Scopes: []string{scopeMCP},
		})(transport).ServeHTTP(w, r)
	})
}
