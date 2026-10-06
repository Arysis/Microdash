package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arysis/microdash/api/internal/alertes"
	"github.com/arysis/microdash/api/internal/calc"
	"github.com/arysis/microdash/api/internal/store"
	"github.com/arysis/microdash/api/internal/stripe"
)

// fraicheurStripe : au-delà, la page relit le compte Stripe. Après un échec, la page ne
// réessaie pas d'elle-même avant pauseStripe, pour ne pas attendre Stripe à chaque affichage.
const (
	fraicheurStripe = 6 * time.Hour
	pauseStripe     = 15 * time.Minute
)

func (s *Server) aujourdhui() time.Time { return alertes.Aujourdhui(s.now()) }

// --- Dépenses récurrentes ---

type recurrenteJSON struct {
	store.Recurrente
	Debut string `json:"debut"`
	Fin   string `json:"fin,omitempty"`
}

func versRecJSON(r store.Recurrente) recurrenteJSON {
	j := recurrenteJSON{Recurrente: r, Debut: r.Debut.Format(formatDate)}
	if r.Fin != nil {
		j.Fin = r.Fin.Format(formatDate)
	}
	return j
}

func (s *Server) validerRecurrente(w http.ResponseWriter, in *recurrenteJSON) bool {
	in.Libelle = strings.TrimSpace(in.Libelle)
	in.Poste = strings.TrimSpace(in.Poste)
	d, err := time.Parse(formatDate, in.Debut)
	switch {
	case in.Libelle == "":
		erreur(w, http.StatusBadRequest, "donne un nom à la dépense (ex. : Figma)")
	case in.Poste == "":
		erreur(w, http.StatusBadRequest, "choisis un poste de dépense")
	case in.Poste == calc.PosteURSSAF:
		erreur(w, http.StatusBadRequest, "un paiement URSSAF change chaque mois : saisis-le dans Saisies, avec sa déclaration")
	case in.Centimes <= 0:
		erreur(w, http.StatusBadRequest, "le montant doit être positif")
	case in.Frequence != calc.Mensuelle && in.Frequence != calc.Annuelle:
		erreur(w, http.StatusBadRequest, "fréquence : mensuelle ou annuelle")
	case err != nil:
		erreur(w, http.StatusBadRequest, "date de première échéance invalide (AAAA-MM-JJ)")
	default:
		in.Recurrente.Debut, in.Recurrente.Fin = d, nil
		if in.Fin != "" {
			f, err := time.Parse(formatDate, in.Fin)
			if err != nil || f.Before(d) {
				erreur(w, http.StatusBadRequest, "date de fin invalide, ou avant la première échéance")
				return false
			}
			in.Recurrente.Fin = &f
		}
		return true
	}
	return false
}

func (s *Server) listRecurrentes(w http.ResponseWriter, r *http.Request) {
	rs, err := s.Store.Recurrentes(r.Context(), userID(r.Context()))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	res := make([]recurrenteJSON, 0, len(rs))
	for _, x := range rs {
		res = append(res, versRecJSON(x))
	}
	ecrireJSON(w, http.StatusOK, res)
}

// enregistrerRecurrente crée ou modifie une dépense, puis crée tout de suite les saisies dues.
func (s *Server) enregistrerRecurrente(w http.ResponseWriter, r *http.Request, id int64) {
	var in recurrenteJSON
	if !lireJSON(w, r, &in) || !s.validerRecurrente(w, &in) {
		return
	}
	uid := userID(r.Context())
	var res *store.Recurrente
	var err error
	if id == 0 {
		res, err = s.Store.CreateRecurrente(r.Context(), uid, in.Recurrente)
	} else {
		in.ID = id
		res, err = s.Store.UpdateRecurrente(r.Context(), uid, in.Recurrente)
	}
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "dépense récurrente introuvable")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	creees, err := s.Store.GenererRecurrentes(r.Context(), uid, s.aujourdhui())
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	code := http.StatusOK
	if id == 0 {
		code = http.StatusCreated
	}
	ecrireJSON(w, code, map[string]any{"recurrente": versRecJSON(*res), "saisies_creees": creees})
}

func (s *Server) createRecurrente(w http.ResponseWriter, r *http.Request) {
	s.enregistrerRecurrente(w, r, 0)
}

func (s *Server) updateRecurrente(w http.ResponseWriter, r *http.Request) {
	if id, ok := idParam(w, r); ok {
		s.enregistrerRecurrente(w, r, id)
	}
}

func (s *Server) deleteRecurrente(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	err := s.Store.DeleteRecurrente(r.Context(), userID(r.Context()), id)
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "dépense récurrente introuvable")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) arreterRecurrente(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	err := s.Store.ArreterRecurrente(r.Context(), userID(r.Context()), id, s.aujourdhui())
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "dépense récurrente introuvable")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Stripe ---

type stripeJSON struct {
	Disponible bool                `json:"disponible"` // le serveur sait chiffrer les clés
	Connecte   bool                `json:"connecte"`
	Mode       string              `json:"mode,omitempty"`
	CleFin     string              `json:"cle_fin,omitempty"`
	CompteID   string              `json:"compte_id,omitempty"`
	CompteNom  string              `json:"compte_nom,omitempty"`
	Depuis     string              `json:"depuis,omitempty"`
	LuLe       string              `json:"lu_le,omitempty"`
	Erreur     string              `json:"erreur,omitempty"`
	MRR        *stripe.MRR         `json:"mrr,omitempty"`
	Mois       []stripe.MoisStripe `json:"mois,omitempty"`
	Tronque    bool                `json:"tronque,omitempty"`
	Prevision  bool                `json:"prevision_mrr"`
}

func contexteCle(uid int64) string { return "stripe:" + strconv.FormatInt(uid, 10) }

func (s *Server) clientStripe(uid int64, c *store.ConnexionStripe) (*stripe.Client, error) {
	cle, err := s.Chiffre.Dechiffrer(c.Cle, contexteCle(uid))
	if err != nil {
		return nil, errors.New("la clé enregistrée ne peut plus être déchiffrée (la clé de chiffrement du serveur a changé) : déconnecte Stripe puis colle à nouveau ta clé")
	}
	return &stripe.Client{Cle: string(cle), URL: s.StripeURL}, nil
}

// etatStripe lit la connexion, relit le compte si la dernière lecture date (ou si force),
// et renvoie l'état à afficher avec la lecture la plus récente.
func (s *Server) etatStripe(ctx context.Context, uid int64, force bool) (*stripeJSON, *stripe.Lecture, error) {
	st := &stripeJSON{Disponible: s.Chiffre != nil}
	if s.Chiffre == nil {
		return st, nil, nil
	}
	c, err := s.Store.ConnexionStripe(ctx, uid)
	if err != nil || c == nil {
		return st, nil, err
	}
	st.Connecte, st.Mode, st.CleFin, st.CompteID, st.CompteNom = true, c.Mode, c.CleFin, c.CompteID, c.CompteNom
	st.Depuis, st.Erreur, st.Prevision = c.Depuis.In(alertes.Paris).Format(formatDate), c.Erreur, c.PrevisionMRR
	var lecture *stripe.Lecture
	if len(c.Donnees) > 0 {
		lecture = &stripe.Lecture{}
		if err := json.Unmarshal(c.Donnees, lecture); err != nil {
			lecture = nil
		}
	}
	perimee := lecture == nil || c.LuLe == nil || s.now().Sub(*c.LuLe) > fraicheurStripe
	if echec, ok := s.echecsStripe.Load(uid); ok && s.now().Sub(echec.(time.Time)) < pauseStripe {
		perimee = false
	}
	if force || perimee {
		cl, err := s.clientStripe(uid, c)
		if err == nil {
			lctx, annuler := context.WithTimeout(ctx, 12*time.Second)
			var nouvelle *stripe.Lecture
			nouvelle, err = cl.Lire(lctx, s.now(), alertes.Paris)
			annuler()
			if err == nil {
				donnees, _ := json.Marshal(nouvelle)
				if err := s.Store.MajLectureStripe(ctx, uid, donnees, ""); err != nil {
					return nil, nil, err
				}
				lecture, st.Erreur = nouvelle, ""
				s.echecsStripe.Delete(uid)
				maintenant := s.now()
				c.LuLe = &maintenant
			}
		}
		if err != nil {
			slog.Warn("lecture Stripe", "compte", uid, "err", err)
			st.Erreur = err.Error()
			s.echecsStripe.Store(uid, s.now())
			if e := s.Store.MajLectureStripe(ctx, uid, nil, st.Erreur); e != nil {
				return nil, nil, e
			}
		}
	}
	if c.LuLe != nil {
		st.LuLe = c.LuLe.In(alertes.Paris).Format(time.RFC3339)
	}
	if lecture != nil {
		st.MRR, st.Mois, st.Tronque = &lecture.MRR, lecture.Mois, lecture.Tronque
	}
	return st, lecture, nil
}

func (s *Server) getStripe(w http.ResponseWriter, r *http.Request) {
	st, _, err := s.etatStripe(r.Context(), userID(r.Context()), false)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, st)
}

func (s *Server) actualiserStripe(w http.ResponseWriter, r *http.Request) {
	st, _, err := s.etatStripe(r.Context(), userID(r.Context()), true)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, st)
}

// putStripe vérifie la clé en lisant le compte, puis l'enregistre chiffrée. Elle n'est jamais renvoyée.
func (s *Server) putStripe(w http.ResponseWriter, r *http.Request) {
	if s.Chiffre == nil {
		erreur(w, http.StatusServiceUnavailable, "la connexion Stripe n'est pas activée sur ce serveur (CLE_CHIFFREMENT manquante)")
		return
	}
	var in struct {
		Cle string `json:"cle"`
	}
	if !lireJSON(w, r, &in) {
		return
	}
	cle := strings.TrimSpace(in.Cle)
	mode, err := stripe.VerifierCle(cle)
	if err != nil {
		erreur(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, annuler := context.WithTimeout(r.Context(), 12*time.Second)
	defer annuler()
	lecture, err := (&stripe.Client{Cle: cle, URL: s.StripeURL}).Lire(ctx, s.now(), alertes.Paris)
	var es *stripe.ErreurStripe
	if errors.As(err, &es) {
		erreur(w, http.StatusBadRequest, "Stripe refuse cette clé : "+es.Message)
		return
	}
	if err != nil {
		erreur(w, http.StatusBadGateway, err.Error())
		return
	}
	uid := userID(r.Context())
	chiffree, err := s.Chiffre.Chiffrer([]byte(cle), contexteCle(uid))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	donnees, _ := json.Marshal(lecture)
	maintenant := s.now()
	err = s.Store.SaveConnexionStripe(r.Context(), uid, store.ConnexionStripe{
		Cle: chiffree, CleFin: cle[len(cle)-4:], Mode: mode, CompteID: lecture.CompteID, CompteNom: lecture.CompteNom,
		Donnees: donnees, LuLe: &maintenant,
	})
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	s.getStripe(w, r)
}

// putPrevisionStripe choisit la source des recettes prévues, puis renvoie la trésorerie recalculée.
func (s *Server) putPrevisionStripe(w http.ResponseWriter, r *http.Request) {
	var in struct {
		MRR bool `json:"mrr"`
	}
	if !lireJSON(w, r, &in) {
		return
	}
	err := s.Store.PrevisionStripe(r.Context(), userID(r.Context()), in.MRR)
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "aucun compte Stripe connecté")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	s.getTresorerie(w, r)
}

func (s *Server) deleteStripe(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteConnexionStripe(r.Context(), userID(r.Context())); err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Trésorerie ---

type soldeJSON struct {
	Centimes *int64 `json:"centimes"`
	Au       string `json:"au,omitempty"`
}

func (s *Server) putSolde(w http.ResponseWriter, r *http.Request) {
	var in soldeJSON
	if !lireJSON(w, r, &in) {
		return
	}
	var so *store.Solde
	if in.Centimes != nil {
		au := s.aujourdhui()
		if in.Au != "" {
			d, err := time.Parse(formatDate, in.Au)
			if err != nil || d.After(au) {
				erreur(w, http.StatusBadRequest, "date du solde invalide, ou dans le futur")
				return
			}
			au = d
		}
		so = &store.Solde{Centimes: *in.Centimes, Au: au}
	}
	if err := s.Store.SaveSolde(r.Context(), userID(r.Context()), so); err != nil {
		erreurInterne(w, r, err)
		return
	}
	s.getTresorerie(w, r)
}

func (s *Server) getTresorerie(w http.ResponseWriter, r *http.Request) {
	p, ok := s.profilRequis(w, r)
	if !ok {
		return
	}
	ctx, uid, auj := r.Context(), userID(r.Context()), s.aujourdhui()
	if _, err := s.Store.GenererRecurrentes(ctx, uid, auj); err != nil {
		erreurInterne(w, r, err)
		return
	}
	const passes, futurs = 11, 6
	debut := time.Date(auj.Year(), auj.Month()-passes-3, 1, 0, 0, 0, 0, time.UTC)
	txs, err := s.Store.Transactions(ctx, uid, debut, auj)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	// Agendas de toutes les années que couvrent les mois affichés.
	var echeances []calc.Echeance
	fin := time.Date(auj.Year(), auj.Month()+futurs, 1, 0, 0, 0, 0, time.UTC)
	for a := auj.AddDate(0, -passes, 0).Year(); a <= fin.Year(); a++ {
		if a < p.DebutActivite.Year() {
			continue
		}
		es, err := alertes.Agenda(ctx, s.Store, s.Baremes, uid, *p, a)
		if err != nil {
			erreur(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		echeances = append(echeances, es...)
	}
	recs, err := s.Store.Recurrentes(ctx, uid)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	solde, err := s.Store.Solde(ctx, uid)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	st, lecture, err := s.etatStripe(ctx, uid, false)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}

	e := calc.EntreeTresorerie{
		Aujourdhui: auj, Profil: p.VersCalc(), Saisies: store.VersCalc(txs), Echeances: echeances,
		MoisPasses: passes, MoisFuturs: futurs,
	}
	for _, x := range recs {
		e.Recurrentes = append(e.Recurrentes, x.VersCalc())
	}
	if lecture != nil && st.Prevision {
		v := lecture.MRR.ParDevise["eur"]
		e.MRR = &v
	}
	var soldeOut *soldeJSON
	if solde != nil {
		e.Solde, e.SoldeAu = &solde.Centimes, solde.Au
		soldeOut = &soldeJSON{Centimes: &solde.Centimes, Au: solde.Au.Format(formatDate)}
	}
	tr, err := calc.CalculerTresorerie(e, s.Baremes)
	if err != nil {
		erreur(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	recJSON := make([]recurrenteJSON, 0, len(recs))
	for _, x := range recs {
		recJSON = append(recJSON, versRecJSON(x))
	}
	ecrireJSON(w, http.StatusOK, map[string]any{
		"aujourdhui": auj.Format(formatDate), "tresorerie": tr, "solde": soldeOut, "stripe": st, "recurrentes": recJSON,
	})
}
