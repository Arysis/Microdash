package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/arysis/microdash/api/internal/ical"
	"github.com/arysis/microdash/api/internal/modules"
	"github.com/arysis/microdash/api/internal/store"
)

// Le flux iCal couvre deux mois passés et un peu plus d'un an à venir.
const (
	fluxAvant  = 60
	fluxApres  = 400
	maxPeriode = 100 // jours lus au plus par /api/calendrier
)

// --- Agenda : tout ce que les modules actifs placent dans le calendrier ---

// evenements rassemble les choses datées des modules actifs, triées par jour puis par heure
// (la journée entière d'abord). Un module en erreur est nommé dans la seconde valeur.
func (s *Server) evenements(ctx context.Context, uid int64, du, au time.Time) ([]modules.Evenement, []string, error) {
	inactifs, err := s.Store.ModulesInactifs(ctx, uid)
	if err != nil {
		return nil, nil, err
	}
	res := []modules.Evenement{}
	indisponibles := []string{}
	for _, mod := range s.Modules.Modules() {
		c, ok := mod.(modules.Calendrier)
		if !ok || inactifs[c.Code()] {
			continue
		}
		evs, err := c.Evenements(ctx, uid, du, au)
		if err != nil {
			slog.Error("agenda : module", "module", c.Code(), "compte", uid, "err", err)
			indisponibles = append(indisponibles, c.Nom())
			continue
		}
		res = append(res, evs...)
	}
	sort.SliceStable(res, func(i, j int) bool {
		a, b := res[i], res[j]
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		return a.Heure < b.Heure
	})
	return res, indisponibles, nil
}

func (s *Server) getCalendrier(w http.ResponseWriter, r *http.Request) {
	du, err1 := time.Parse(formatDate, r.URL.Query().Get("du"))
	au, err2 := time.Parse(formatDate, r.URL.Query().Get("au"))
	if err1 != nil || err2 != nil || au.Before(du) || au.Sub(du) > maxPeriode*24*time.Hour {
		erreur(w, http.StatusBadRequest, "période invalide : du=AAAA-MM-JJ&au=AAAA-MM-JJ, 100 jours au plus")
		return
	}
	evs, indisponibles, err := s.evenements(r.Context(), userID(r.Context()), du, au)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]any{
		"aujourdhui": s.aujourdhui().Format(formatDate), "evenements": evs, "indisponibles": indisponibles,
	})
}

// --- Rendez-vous du module Agenda ---

func validerEvenement(w http.ResponseWriter, e *store.Evenement) bool {
	e.Titre = strings.TrimSpace(e.Titre)
	e.Notes = strings.TrimSpace(e.Notes)
	_, errJour := time.Parse(formatDate, e.Jour)
	_, errHeure := time.Parse("15:04", e.Heure)
	switch {
	case e.Titre == "":
		erreur(w, http.StatusBadRequest, "donne un titre au rendez-vous")
	case utf8.RuneCountInString(e.Titre) > 200:
		erreur(w, http.StatusBadRequest, "titre trop long : 200 caractères au plus, le reste va dans les notes")
	case utf8.RuneCountInString(e.Notes) > 4000:
		erreur(w, http.StatusBadRequest, "notes trop longues : 4 000 caractères au plus")
	case errJour != nil:
		erreur(w, http.StatusBadRequest, "jour invalide (AAAA-MM-JJ)")
	case e.Heure != "" && errHeure != nil:
		erreur(w, http.StatusBadRequest, "heure invalide (HH:MM)")
	case e.Duree < 0 || e.Duree > 1440:
		erreur(w, http.StatusBadRequest, "durée entre 1 minute et 24 heures")
	default:
		if e.Heure == "" {
			e.Duree = 0 // une journée entière n'a pas de durée
		}
		return true
	}
	return false
}

func (s *Server) createEvenement(w http.ResponseWriter, r *http.Request) {
	var in store.Evenement
	if !lireJSON(w, r, &in) || !validerEvenement(w, &in) {
		return
	}
	e, err := s.Store.CreateEvenement(r.Context(), userID(r.Context()), in)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusCreated, e)
}

func (s *Server) updateEvenement(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	var in store.Evenement
	if !lireJSON(w, r, &in) || !validerEvenement(w, &in) {
		return
	}
	in.ID = id
	e, err := s.Store.UpdateEvenement(r.Context(), userID(r.Context()), in)
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "rendez-vous introuvable")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, e)
}

func (s *Server) deleteEvenement(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	err := s.Store.DeleteEvenement(r.Context(), userID(r.Context()), id)
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "rendez-vous introuvable")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Flux iCal (Home Assistant, téléphone) ---

func (s *Server) getFlux(w http.ResponseWriter, r *http.Request) {
	depuis, err := s.Store.FluxICal(r.Context(), userID(r.Context()))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	res := map[string]any{"actif": depuis != nil}
	if depuis != nil {
		res["cree_le"] = depuis.Format(time.RFC3339)
	}
	ecrireJSON(w, http.StatusOK, res)
}

// changerFlux crée un nouveau lien secret, et renvoie son chemin : c'est la seule fois où il
// est visible. L'ancien lien cesse de marcher.
func (s *Server) changerFlux(w http.ResponseWriter, r *http.Request) {
	brut := make([]byte, 32)
	if _, err := rand.Read(brut); err != nil {
		erreurInterne(w, r, err)
		return
	}
	jeton := base64.RawURLEncoding.EncodeToString(brut)
	if err := s.Store.ChangerFluxICal(r.Context(), userID(r.Context()), hacherJeton(jeton)); err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]any{"actif": true, "chemin": "/api/ical/" + jeton + ".ics"})
}

func (s *Server) supprimerFlux(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.SupprimerFluxICal(r.Context(), userID(r.Context())); err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// fluxICal sert le calendrier au format iCalendar à qui connaît le lien secret.
func (s *Server) fluxICal(w http.ResponseWriter, r *http.Request) {
	jeton := chi.URLParam(r, "jeton")
	if len(jeton) != 43 {
		http.NotFound(w, r)
		return
	}
	uid, err := s.Store.CompteDuFlux(r.Context(), hacherJeton(jeton))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	aujourdhui := s.aujourdhui()
	evs, _, err := s.evenements(r.Context(), uid, aujourdhui.AddDate(0, 0, -fluxAvant), aujourdhui.AddDate(0, 0, fluxApres))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	entrees := make([]ical.Evenement, 0, len(evs))
	for _, e := range evs {
		jour, err := time.Parse(formatDate, e.Date)
		if err != nil {
			continue
		}
		titre := e.Titre
		switch e.Type {
		case "tache":
			titre = "Tâche : " + titre
		case "echeance":
			titre = "À payer ou déclarer : " + titre
		}
		if e.Fait {
			titre += " (fait)"
		}
		entrees = append(entrees, ical.Evenement{
			UID: e.Source + "-" + e.Ref + "@microdash", Titre: titre, Notes: e.Notes,
			Date: jour, Heure: e.Heure, Duree: e.Duree,
		})
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Robots-Tag", "noindex")
	_, _ = w.Write([]byte(ical.Ecrire("Microdash", entrees, s.now())))
}
