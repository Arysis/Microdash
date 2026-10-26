package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/arysis/microdash/api/internal/gmail"
	"github.com/arysis/microdash/api/internal/modules/mails"
	"github.com/arysis/microdash/api/internal/store"
)

// --- Mails (Gmail) ---

type mailJSON struct {
	store.Mail
	Sujet     string `json:"sujet"`
	Important bool   `json:"important"`
	Raison    string `json:"raison,omitempty"`
	Lien      string `json:"lien"`
}

type mailsJSON struct {
	Disponible     bool              `json:"disponible"` // le serveur sait chiffrer le mot de passe
	Connecte       bool              `json:"connecte"`
	Email          string            `json:"email,omitempty"`
	ImportantGmail bool              `json:"important_gmail"`
	SynchroLe      *time.Time        `json:"synchro_le,omitempty"`
	Erreur         string            `json:"erreur,omitempty"`
	Mails          []mailJSON        `json:"mails"`
	Regles         []store.RegleMail `json:"regles"`
}

func (s *Server) etatMails(ctx context.Context, uid int64) (*mailsJSON, error) {
	res := &mailsJSON{Disponible: s.Chiffre != nil, Mails: []mailJSON{}, Regles: []store.RegleMail{}}
	c, err := s.Store.ConnexionMail(ctx, uid)
	if err != nil {
		return nil, err
	}
	if res.Regles, err = s.Store.ReglesMail(ctx, uid); err != nil {
		return nil, err
	}
	if c == nil {
		return res, nil
	}
	res.Connecte, res.Email, res.ImportantGmail, res.SynchroLe, res.Erreur = true, c.Email, c.ImportantGmail, c.SynchroLe, c.Erreur
	ms, err := s.Store.Mails(ctx, uid)
	if err != nil {
		return nil, err
	}
	for _, m := range ms {
		imp, raison := mails.Important(m, res.Regles, c.ImportantGmail)
		res.Mails = append(res.Mails, mailJSON{Mail: m, Sujet: mails.Sujet(m), Important: imp, Raison: raison, Lien: mails.LienGmail(m.MessageID)})
	}
	return res, nil
}

func (s *Server) getMails(w http.ResponseWriter, r *http.Request) {
	res, err := s.etatMails(r.Context(), userID(r.Context()))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, res)
}

// putConnexionMail vérifie l'adresse et le mot de passe d'application auprès de Gmail, les
// enregistre, puis lance une première synchro.
func (s *Server) putConnexionMail(w http.ResponseWriter, r *http.Request) {
	if s.Chiffre == nil {
		erreur(w, http.StatusServiceUnavailable, "la connexion Gmail n'est pas activée sur ce serveur (CLE_CHIFFREMENT manquante)")
		return
	}
	var in struct {
		Email          string `json:"email"`
		MotDePasse     string `json:"mot_de_passe"`
		ImportantGmail bool   `json:"important_gmail"`
	}
	if !lireJSON(w, r, &in) {
		return
	}
	email, ok := normaliserEmail(in.Email)
	// Google affiche le mot de passe d'application par groupes de quatre lettres.
	mdp := strings.ReplaceAll(strings.TrimSpace(in.MotDePasse), " ", "")
	switch {
	case !ok:
		erreur(w, http.StatusBadRequest, "adresse Gmail invalide")
		return
	case len(mdp) != 16:
		erreur(w, http.StatusBadRequest, "un mot de passe d'application Google fait 16 lettres (ex. : abcd efgh ijkl mnop)")
		return
	}
	ctx, annuler := context.WithTimeout(r.Context(), 12*time.Second)
	err := gmail.Compte{Email: email, MotDePasse: mdp}.Verifier(ctx)
	annuler()
	if errors.Is(err, gmail.ErrIdentifiants) {
		erreur(w, http.StatusBadRequest, "Gmail refuse cette adresse ou ce mot de passe d'application. Vérifie que l'IMAP est activé dans Gmail.")
		return
	}
	if err != nil {
		erreur(w, http.StatusBadGateway, err.Error())
		return
	}
	uid := userID(r.Context())
	chiffre, err := s.Chiffre.Chiffrer([]byte(mdp), mails.ContexteCle(uid))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	if err := s.Store.SaveConnexionMail(r.Context(), uid, store.ConnexionMail{Email: email, MotDePasse: chiffre, ImportantGmail: in.ImportantGmail}); err != nil {
		erreurInterne(w, r, err)
		return
	}
	// La première synchro peut dépasser le délai d'une requête : elle continue sans elle.
	go func() {
		ctx, annuler := context.WithTimeout(context.Background(), time.Minute)
		defer annuler()
		_ = s.Mails.Synchroniser(ctx, uid, s.now())
	}()
	s.getMails(w, r)
}

func (s *Server) deleteConnexionMail(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.SupprimerConnexionMail(r.Context(), userID(r.Context())); err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) putImportantGmail(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Actif bool `json:"actif"`
	}
	if !lireJSON(w, r, &in) {
		return
	}
	if err := s.Store.ImportantGmail(r.Context(), userID(r.Context()), in.Actif); err != nil {
		erreurInterne(w, r, err)
		return
	}
	s.getMails(w, r)
}

func (s *Server) synchroMails(w http.ResponseWriter, r *http.Request) {
	uid := userID(r.Context())
	if err := s.Mails.Synchroniser(r.Context(), uid, s.now()); err != nil {
		// L'erreur est notée sur la connexion : la page l'affiche.
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			erreur(w, http.StatusGatewayTimeout, "Gmail met trop de temps à répondre, réessaie dans un moment")
			return
		}
	}
	s.getMails(w, r)
}

// archiverMail sort un mail de la boîte de réception Gmail.
func (s *Server) archiverMail(w http.ResponseWriter, r *http.Request) {
	ctx, uid, ref := r.Context(), userID(r.Context()), chi.URLParam(r, "ref")
	if err := s.Mails.Finir(ctx, uid, ref); err != nil {
		erreur(w, http.StatusBadGateway, "archivage impossible : "+err.Error())
		return
	}
	if err := s.Store.MarquerATraiter(ctx, uid, s.Mails.Code(), ref, nil); err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// normaliserMotif accepte une adresse (paul@exemple.fr) ou un domaine (@exemple.fr).
func normaliserMotif(m string) (string, bool) {
	m = strings.ToLower(strings.TrimSpace(m))
	if strings.HasPrefix(m, "@") {
		d := m[1:]
		return m, len(d) >= 3 && strings.Contains(d, ".") && !strings.ContainsAny(d, "@ ,;<>")
	}
	return normaliserEmail(m)
}

func (s *Server) createRegleMail(w http.ResponseWriter, r *http.Request) {
	var in store.RegleMail
	if !lireJSON(w, r, &in) {
		return
	}
	motif, ok := normaliserMotif(in.Motif)
	if !ok {
		erreur(w, http.StatusBadRequest, "écris une adresse (paul@exemple.fr) ou un domaine (@exemple.fr)")
		return
	}
	in.Motif, in.Libelle = motif, strings.TrimSpace(in.Libelle)
	if len([]rune(in.Libelle)) > 60 {
		erreur(w, http.StatusBadRequest, "nom de la règle trop long : 60 caractères au plus")
		return
	}
	res, err := s.Store.CreateRegleMail(r.Context(), userID(r.Context()), in)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusCreated, res)
}

func (s *Server) deleteRegleMail(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	err := s.Store.DeleteRegleMail(r.Context(), userID(r.Context()), id)
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "règle introuvable")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
