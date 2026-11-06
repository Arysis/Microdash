package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/arysis/microdash/api/internal/store"
)

// Réinitialisation temporaire du mot de passe, sans e-mail : l'e-mail et le nouveau mot de passe
// suffisent. Elle n'est ouverte qu'aux adresses de MdpOublie (« * » : toutes). Liste vide : coupée.

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
	ecrireJSON(w, http.StatusOK, map[string]bool{"actif": len(s.MdpOublie) > 0})
}

func (s *Server) mdpOublie(w http.ResponseWriter, r *http.Request) {
	var in identifiants
	if !lireJSON(w, r, &in) {
		return
	}
	if len(s.MdpOublie) == 0 {
		erreur(w, http.StatusNotFound, "la réinitialisation du mot de passe est désactivée")
		return
	}
	email, _ := normaliserEmail(in.Email)
	if len(in.MotDePasse) < 10 {
		erreur(w, http.StatusBadRequest, "le mot de passe doit faire au moins 10 caractères")
		return
	}
	impossible := "réinitialisation impossible pour cette adresse"
	if !s.mdpOublieAutorise(email) {
		erreur(w, http.StatusForbidden, impossible)
		return
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
	if err := s.Store.ChangerMotDePasse(r.Context(), u.ID, hash); err != nil {
		erreurInterne(w, r, err)
		return
	}
	if s.ouvrirSession(w, r, u.ID) {
		s.ecrireMe(w, r, u)
	}
}
