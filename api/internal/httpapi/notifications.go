package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/arysis/microdash/api/internal/notifs"
	"github.com/arysis/microdash/api/internal/store"
	"github.com/arysis/microdash/api/internal/webpush"
)

// --- Notifications push (PC, téléphone, PWA installée) ---

var avancesPermises = map[int]bool{0: true, 5: true, 10: true, 15: true, 30: true, 60: true, 120: true}

func (s *Server) getNotifications(w http.ResponseWriter, r *http.Request) {
	if s.Notifs == nil {
		ecrireJSON(w, http.StatusOK, map[string]any{"disponible": false})
		return
	}
	ctx, uid := r.Context(), userID(r.Context())
	p, err := s.Store.PreferencesNotifications(ctx, uid)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	abs, err := s.Store.Abonnements(ctx, uid)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]any{
		"disponible": true, "cle_publique": s.Notifs.ClePublique, "preferences": p, "appareils": abs,
	})
}

type abonnementJSON struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	Appareil string `json:"appareil"`
}

func (s *Server) abonnerNotifications(w http.ResponseWriter, r *http.Request) {
	if s.Notifs == nil {
		erreur(w, http.StatusServiceUnavailable, "notifications indisponibles sur ce serveur")
		return
	}
	var in abonnementJSON
	if !lireJSON(w, r, &in) {
		return
	}
	in.Appareil = strings.TrimSpace(in.Appareil)
	switch {
	case len(in.Endpoint) > 2000 || !webpush.HoteAutorise(in.Endpoint):
		erreur(w, http.StatusBadRequest, "service de notifications du navigateur non reconnu")
		return
	case in.Keys.P256dh == "" || in.Keys.Auth == "" || len(in.Keys.P256dh) > 200 || len(in.Keys.Auth) > 100:
		erreur(w, http.StatusBadRequest, "clés de l'abonnement manquantes")
		return
	case utf8.RuneCountInString(in.Appareil) > 80:
		in.Appareil = string([]rune(in.Appareil)[:80])
	}
	if in.Appareil == "" {
		in.Appareil = "Appareil"
	}
	id, err := s.Store.Abonner(r.Context(), store.AbonnementPush{
		UserID: userID(r.Context()), Endpoint: in.Endpoint, P256dh: in.Keys.P256dh, Auth: in.Keys.Auth, Appareil: in.Appareil,
	})
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusCreated, map[string]any{"id": id})
}

// desabonnerNotifications retire l'appareil courant, désigné par son adresse de push.
func (s *Server) desabonnerNotifications(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Endpoint string `json:"endpoint"`
	}
	if !lireJSON(w, r, &in) {
		return
	}
	if err := s.Store.Desabonner(r.Context(), userID(r.Context()), in.Endpoint); err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) supprimerAppareil(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	err := s.Store.SupprimerAbonnement(r.Context(), userID(r.Context()), id)
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "appareil introuvable")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) putPreferencesNotifications(w http.ResponseWriter, r *http.Request) {
	var p store.PreferencesNotifications
	if !lireJSON(w, r, &p) {
		return
	}
	if _, err := time.Parse("15:04", p.HeureSoiree); err != nil {
		erreur(w, http.StatusBadRequest, "heure du résumé invalide (HH:MM)")
		return
	}
	if !avancesPermises[p.Avance] {
		erreur(w, http.StatusBadRequest, "rappel : 0, 5, 10, 15, 30, 60 ou 120 minutes avant")
		return
	}
	if err := s.Store.ChangerPreferencesNotifications(r.Context(), userID(r.Context()), p); err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, p)
}

// essaiNotification envoie une notification à tous les appareils du compte.
func (s *Server) essaiNotification(w http.ResponseWriter, r *http.Request) {
	if s.Notifs == nil {
		erreur(w, http.StatusServiceUnavailable, "notifications indisponibles sur ce serveur")
		return
	}
	n := notifs.Notification{
		Titre: "Microdash", Corps: "Les notifications marchent sur cet appareil.",
		Lien: "/compte", Tag: "essai", TTL: 10 * time.Minute, Urgence: "high",
	}
	atteints, err := s.Notifs.Pousser(r.Context(), userID(r.Context()), n)
	if err != nil {
		erreur(w, http.StatusBadGateway, "envoi impossible : "+err.Error())
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]any{"appareils": atteints})
}
