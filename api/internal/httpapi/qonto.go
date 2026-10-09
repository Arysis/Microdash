package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/arysis/microdash/api/internal/alertes"
	"github.com/arysis/microdash/api/internal/calc"
	"github.com/arysis/microdash/api/internal/qonto"
	"github.com/arysis/microdash/api/internal/store"
)

// --- Qonto ---

type qontoJSON struct {
	Disponible   bool   `json:"disponible"` // le serveur sait chiffrer les clés
	Connecte     bool   `json:"connecte"`
	Identifiant  string `json:"identifiant,omitempty"`
	CleFin       string `json:"cle_fin,omitempty"`
	Organisation string `json:"organisation,omitempty"`
	Depuis       string `json:"depuis,omitempty"`
	SynchroLe    string `json:"synchro_le,omitempty"`
	Erreur       string `json:"erreur,omitempty"`
	AValider     int    `json:"a_valider"`
}

func contexteCleQonto(uid int64) string { return "qonto:" + strconv.FormatInt(uid, 10) }

func (s *Server) etatQonto(ctx context.Context, uid int64) (*qontoJSON, error) {
	st := &qontoJSON{Disponible: s.Chiffre != nil}
	n, err := s.Store.CompterAValider(ctx, uid)
	if err != nil {
		return nil, err
	}
	st.AValider = n
	if s.Chiffre == nil {
		return st, nil
	}
	c, err := s.Store.ConnexionQonto(ctx, uid)
	if err != nil || c == nil {
		return st, err
	}
	st.Connecte, st.Identifiant, st.CleFin, st.Organisation, st.Erreur = true, c.Identifiant, c.CleFin, c.Organisation, c.Erreur
	st.Depuis = c.Depuis.In(alertes.Paris).Format(formatDate)
	if c.SynchroLe != nil {
		st.SynchroLe = c.SynchroLe.In(alertes.Paris).Format(time.RFC3339)
	}
	return st, nil
}

func (s *Server) getQonto(w http.ResponseWriter, r *http.Request) {
	st, err := s.etatQonto(r.Context(), userID(r.Context()))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, st)
}

// erreurSynchro est une erreur à montrer telle quelle (clé refusée, Qonto injoignable, profil manquant).
type erreurSynchro struct{ msg string }

func (e erreurSynchro) Error() string { return e.msg }

// synchroniserQonto lit les opérations réglées depuis le 1er janvier (seulement celles modifiées
// depuis la dernière synchro, s'il y en a eu une), les range, et note le solde des comptes.
func (s *Server) synchroniserQonto(ctx context.Context, uid int64, cl *qonto.Client, derniere *time.Time) (store.BilanImport, error) {
	var b store.BilanImport
	p, err := s.Store.Profil(ctx, uid)
	if errors.Is(err, store.ErrNotFound) {
		return b, erreurSynchro{"renseigne ton profil avant de synchroniser Qonto"}
	}
	if err != nil {
		return b, err
	}
	debut, auj := s.now(), s.aujourdhui()
	lctx, annuler := context.WithTimeout(ctx, 12*time.Second)
	defer annuler()
	org, err := cl.Organisation(lctx)
	if err != nil {
		return b, erreurSynchro{err.Error()}
	}
	ibans := map[string]bool{}
	for _, c := range org.Comptes {
		if c.IBAN != "" {
			ibans[qonto.NormaliserIBAN(c.IBAN)] = true
		}
	}
	depuis := time.Date(auj.Year(), 1, 1, 0, 0, 0, 0, alertes.Paris)
	var majDepuis *time.Time
	if derniere != nil {
		m := derniere.Add(-48 * time.Hour) // marge : une opération réglée tard garde son ancienne date
		majDepuis = &m
	}
	var ops []store.OperationQonto
	var solde int64
	comptes := org.ComptesSuivis()
	for _, c := range comptes {
		solde += c.Solde
		lues, _, err := cl.Operations(lctx, c.ID, depuis, majDepuis)
		if err != nil {
			return b, erreurSynchro{err.Error()}
		}
		for _, o := range lues {
			if o.RegleLe == nil || o.Montant <= 0 || (o.Devise != "" && !strings.EqualFold(o.Devise, "EUR")) {
				continue
			}
			d := o.RegleLe.In(alertes.Paris)
			op := store.OperationQonto{
				QontoID: o.ID, Date: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC), Centimes: o.Montant,
				Type: calc.Recette, Nom: o.Nom(), Reference: strings.TrimSpace(o.Reference), TypeOperation: o.Type,
				Cle: qonto.CleLibelle(o), Interne: qonto.Interne(o, ibans),
			}
			if o.Sens == "debit" {
				op.Type = calc.Depense
				op.EcheanceProposee = qonto.DeclarationPrecedente(op.Date, p.Periodicite)
				switch {
				case qonto.EstURSSAF(o):
					op.PostePropose = calc.PosteURSSAF
				case o.Type == "qonto_fee":
					op.PostePropose = "Frais bancaires"
				}
			}
			ops = append(ops, op)
		}
	}
	if _, err := s.Store.GenererRecurrentes(ctx, uid, auj); err != nil {
		return b, err
	}
	b, err = s.Store.ImporterQonto(ctx, uid, ops, categoriesProfil(p))
	if err != nil {
		return b, err
	}
	if len(comptes) > 0 {
		if err := s.Store.SaveSolde(ctx, uid, &store.Solde{Centimes: solde, Au: auj}); err != nil {
			return b, err
		}
	}
	return b, s.Store.MajSynchroQonto(ctx, uid, debut, "")
}

func categoriesProfil(p *store.Profil) []string {
	res := []string{p.Categorie}
	if p.CategorieSecondaire != "" {
		res = append(res, p.CategorieSecondaire)
	}
	return res
}

// lancerSynchro synchronise et répond avec l'état et le bilan, ou l'erreur lisible.
func (s *Server) lancerSynchro(w http.ResponseWriter, r *http.Request, cl *qonto.Client, derniere *time.Time) {
	ctx, uid := r.Context(), userID(r.Context())
	b, err := s.synchroniserQonto(ctx, uid, cl, derniere)
	var es erreurSynchro
	if errors.As(err, &es) {
		slog.Warn("synchro Qonto", "compte", uid, "err", err)
		if e := s.Store.MajSynchroQonto(ctx, uid, time.Time{}, es.msg); e != nil {
			erreurInterne(w, r, e)
			return
		}
		erreur(w, http.StatusBadGateway, es.msg)
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	st, err := s.etatQonto(ctx, uid)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]any{"qonto": st, "bilan": b})
}

func (s *Server) synchroQonto(w http.ResponseWriter, r *http.Request) {
	ctx, uid := r.Context(), userID(r.Context())
	if s.Chiffre == nil {
		erreur(w, http.StatusServiceUnavailable, "la connexion Qonto n'est pas activée sur ce serveur (CLE_CHIFFREMENT manquante)")
		return
	}
	c, err := s.Store.ConnexionQonto(ctx, uid)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	if c == nil {
		erreur(w, http.StatusNotFound, "aucun compte Qonto connecté")
		return
	}
	cle, err := s.Chiffre.Dechiffrer(c.Cle, contexteCleQonto(uid))
	if err != nil {
		erreur(w, http.StatusConflict, "la clé enregistrée ne peut plus être déchiffrée (la clé de chiffrement du serveur a changé) : déconnecte Qonto puis colle à nouveau ta clé")
		return
	}
	s.lancerSynchro(w, r, &qonto.Client{Identifiant: c.Identifiant, Cle: string(cle), URL: s.QontoURL}, c.SynchroLe)
}

// putQonto vérifie la clé en lisant l'organisation, l'enregistre chiffrée, puis synchronise.
func (s *Server) putQonto(w http.ResponseWriter, r *http.Request) {
	if s.Chiffre == nil {
		erreur(w, http.StatusServiceUnavailable, "la connexion Qonto n'est pas activée sur ce serveur (CLE_CHIFFREMENT manquante)")
		return
	}
	var in struct {
		Identifiant string `json:"identifiant"`
		Cle         string `json:"cle"`
	}
	if !lireJSON(w, r, &in) {
		return
	}
	id, cle := strings.TrimSpace(in.Identifiant), strings.TrimSpace(in.Cle)
	if err := qonto.VerifierCle(id, cle); err != nil {
		erreur(w, http.StatusBadRequest, err.Error())
		return
	}
	cl := &qonto.Client{Identifiant: id, Cle: cle, URL: s.QontoURL}
	ctx, annuler := context.WithTimeout(r.Context(), 8*time.Second)
	org, err := cl.Organisation(ctx)
	annuler()
	var eq *qonto.ErreurQonto
	if errors.As(err, &eq) {
		erreur(w, http.StatusBadRequest, "Qonto refuse cette clé : "+eq.Message)
		return
	}
	if err != nil {
		erreur(w, http.StatusBadGateway, err.Error())
		return
	}
	uid := userID(r.Context())
	chiffree, err := s.Chiffre.Chiffrer([]byte(cle), contexteCleQonto(uid))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	err = s.Store.SaveConnexionQonto(r.Context(), uid, store.ConnexionQonto{
		Identifiant: id, Cle: chiffree, CleFin: cle[len(cle)-4:], Organisation: org.Libelle(),
	})
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	s.lancerSynchro(w, r, cl, nil)
}

func (s *Server) deleteQonto(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteConnexionQonto(r.Context(), userID(r.Context())); err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Opérations à valider ---

type operationJSON struct {
	store.OperationQonto
	Date string `json:"date"`
}

func (s *Server) listOperationsQonto(w http.ResponseWriter, r *http.Request) {
	statut := r.URL.Query().Get("statut")
	if statut == "" {
		statut = "a_valider"
	}
	if statut != "a_valider" && statut != "ignoree" {
		erreur(w, http.StatusBadRequest, "statut : a_valider ou ignoree")
		return
	}
	ops, err := s.Store.OperationsQonto(r.Context(), userID(r.Context()), statut)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	res := make([]operationJSON, 0, len(ops))
	for _, o := range ops {
		res = append(res, operationJSON{OperationQonto: o, Date: o.Date.Format(formatDate)})
	}
	ecrireJSON(w, http.StatusOK, res)
}

func erreurOperation(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		erreur(w, http.StatusNotFound, "opération introuvable")
	case errors.Is(err, store.ErrDejaTraitee):
		erreur(w, http.StatusConflict, "cette opération a déjà été traitée")
	default:
		erreurInterne(w, r, err)
	}
}

// validerOperationQonto crée la saisie (montant, date, poste… modifiables) d'une opération.
func (s *Server) validerOperationQonto(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	var in struct {
		transactionJSON
		Retenir bool `json:"retenir"`
	}
	if !lireJSON(w, r, &in) || !s.validerTransaction(w, r, &in.transactionJSON) {
		return
	}
	t, err := s.Store.ValiderOperationQonto(r.Context(), userID(r.Context()), id, in.Transaction, in.Retenir)
	if err != nil {
		erreurOperation(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusCreated, versJSON(*t))
}

func (s *Server) ignorerOperationQonto(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	var in struct {
		Retenir bool `json:"retenir"`
	}
	if !lireJSON(w, r, &in) {
		return
	}
	if err := s.Store.IgnorerOperationQonto(r.Context(), userID(r.Context()), id, in.Retenir); err != nil {
		erreurOperation(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) remettreOperationQonto(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	if err := s.Store.RemettreOperationQonto(r.Context(), userID(r.Context()), id); err != nil {
		erreurOperation(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
