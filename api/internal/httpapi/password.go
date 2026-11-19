package httpapi

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTemps    = 2
	argonMemoire  = 64 * 1024
	argonFils     = 2
	argonLongueur = 32
)

func hashPassword(pw string) (string, error) {
	sel := make([]byte, 16)
	if _, err := rand.Read(sel); err != nil {
		return "", err
	}
	h := argon2.IDKey([]byte(pw), sel, argonTemps, argonMemoire, argonFils, argonLongueur)
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemoire, argonTemps, argonFils, b64.EncodeToString(sel), b64.EncodeToString(h)), nil
}

func checkPassword(pw, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var m uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	b64 := base64.RawStdEncoding
	sel, err1 := b64.DecodeString(parts[4])
	attendu, err2 := b64.DecodeString(parts[5])
	if err1 != nil || err2 != nil {
		return false
	}
	h := argon2.IDKey([]byte(pw), sel, t, m, p, uint32(len(attendu)))
	return subtle.ConstantTimeCompare(h, attendu) == 1
}

type changementMotDePasse struct {
	Actuel  string `json:"actuel"`
	Nouveau string `json:"nouveau"`
}

// changerMotDePasse : connecté, avec le mot de passe actuel. Ferme les autres sessions (et
// l'accès de Claude), puis rouvre celle de cet appareil.
func (s *Server) changerMotDePasse(w http.ResponseWriter, r *http.Request) {
	var in changementMotDePasse
	if !lireJSON(w, r, &in) {
		return
	}
	if len(in.Nouveau) < 10 {
		erreur(w, http.StatusBadRequest, "le nouveau mot de passe doit faire au moins 10 caractères")
		return
	}
	u, err := s.Store.UserByID(r.Context(), userID(r.Context()))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	if !checkPassword(in.Actuel, u.PasswordHash) {
		erreur(w, http.StatusForbidden, "mot de passe actuel incorrect")
		return
	}
	hash, err := hashPassword(in.Nouveau)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	if err := s.Store.ChangerMotDePasse(r.Context(), u.ID, hash); err != nil {
		erreurInterne(w, r, err)
		return
	}
	if s.ouvrirSession(w, r, u.ID) {
		s.ecrireMe(w, r, u)
	}
}
