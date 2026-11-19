package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/arysis/microdash/api/internal/store"
)

// Réinitialisation temporaire du mot de passe, sans e-mail : l'e-mail et le nouveau mot de passe
// suffisent. Elle n'est ouverte qu'aux adresses de MdpOublie (« * » : toutes). Liste vide : coupée.

// CodeSecours : empreinte argon2id d'un code de secours à usage unique, pour réinitialiser un
// mot de passe sans e-mail ni accès au serveur. Le code lui-même n'est jamais dans le dépôt.
// Temporaire : à retirer quand l'envoi d'e-mails (Resend) marche en prod.
const CodeSecours = "$argon2id$v=19$m=65536,t=2,p=2$+UqKgT+Ir3SF8l1KOIEDuQ$HYnXponvjD7T0Vjiah9jlZSarzHQhbcLzdjo4bhL0VM"

// normaliserCode : majuscules, sans espaces ni tirets, pour accepter « ambrc-uuvcj… » comme « AMBRCUUVCJ… ».
func normaliserCode(c string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(c) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (s *Server) mdpOublieAutorise(email string) bool {
	for _, e := range s.MdpOublie {
		if e == "*" || e == email {
			return true
		}
	}
	return false
}

// ListeMdpOublie lit MDP_OUBLIE_EMAILS : adresses séparées par des virgules, ou « * ».
func ListeMdpOublie(v string) []string {
	var liste []string
	for _, e := range strings.Split(v, ",") {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			liste = append(liste, e)
		}
	}
	return liste
}

func (s *Server) etatMdpOublie(w http.ResponseWriter, r *http.Request) {
	ecrireJSON(w, http.StatusOK, map[string]bool{"actif": len(s.MdpOublie) > 0 || s.CodeSecours != "", "code": s.CodeSecours != ""})
}

func (s *Server) mdpOublie(w http.ResponseWriter, r *http.Request) {
	var in struct {
		identifiants
		Code string `json:"code"`
	}
	if !lireJSON(w, r, &in) {
		return
	}
	if len(s.MdpOublie) == 0 && s.CodeSecours == "" {
		erreur(w, http.StatusNotFound, "la réinitialisation du mot de passe est désactivée")
		return
	}
	email, _ := normaliserEmail(in.Email)
	if len(in.MotDePasse) < 10 {
		erreur(w, http.StatusBadRequest, "le mot de passe doit faire au moins 10 caractères")
		return
	}
	impossible := "réinitialisation impossible pour cette adresse"
	// Sans code : seulement les adresses de MDP_OUBLIE_EMAILS. Avec un code : le bon code, une fois.
	avecCode := !s.mdpOublieAutorise(email)
	if avecCode {
		if in.Code == "" || s.CodeSecours == "" {
			erreur(w, http.StatusForbidden, impossible)
			return
		}
		if !checkPassword(normaliserCode(in.Code), s.CodeSecours) {
			erreur(w, http.StatusForbidden, "code de secours incorrect")
			return
		}
	}
	u, err := s.Store.UserByEmail(r.Context(), email)
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusForbidden, impossible)
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	hash, err := hashPassword(in.MotDePasse)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	// Ferme toutes les sessions ouvertes avec l'ancien mot de passe, puis connecte ici.
	if avecCode {
		err = s.Store.ChangerMotDePasseAvecCode(r.Context(), u.ID, hash, s.CodeSecours)
	} else {
		err = s.Store.ChangerMotDePasse(r.Context(), u.ID, hash)
	}
	if errors.Is(err, store.ErrCodeUtilise) {
		erreur(w, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	if s.ouvrirSession(w, r, u.ID) {
		s.ecrireMe(w, r, u)
	}
}
