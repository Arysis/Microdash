package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/arysis/microdash/api/internal/modules"
	"github.com/arysis/microdash/api/internal/modules/taches"
	"github.com/arysis/microdash/api/internal/store"
)

// --- Tâches ---

type tacheJSON struct {
	store.Tache
	Faite bool `json:"faite"`
}

func versTacheJSON(t store.Tache) tacheJSON { return tacheJSON{Tache: t, Faite: t.FaiteLe != ""} }

func (s *Server) validerTache(w http.ResponseWriter, in *tacheJSON) bool {
	in.Titre = strings.TrimSpace(in.Titre)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.Quand == "" {
		in.Quand = taches.Semaine
	}
	_, projetConnu := taches.Projets[in.Projet]
	switch {
	case in.Titre == "":
		erreur(w, http.StatusBadRequest, "écris la tâche (ex. : Relancer Paul)")
	case utf8.RuneCountInString(in.Titre) > 200:
		erreur(w, http.StatusBadRequest, "tâche trop longue : 200 caractères au plus, le reste va dans les notes")
	case utf8.RuneCountInString(in.Notes) > 4000:
		erreur(w, http.StatusBadRequest, "notes trop longues : 4 000 caractères au plus")
	case in.Projet != "" && !projetConnu:
		erreur(w, http.StatusBadRequest, "projet inconnu : educari, saas, microdash ou perso")
	case in.Duree < 0 || in.Duree > 1440:
		erreur(w, http.StatusBadRequest, "durée entre 1 minute et 24 heures")
	default:
		var err error
		if in.Debut, in.Fin, err = taches.Periode(in.Quand, in.Debut, in.Fin, s.aujourdhui()); err != nil {
			erreur(w, http.StatusBadRequest, err.Error())
			return false
		}
		if in.Echeance != "" {
			if _, err := time.Parse(formatDate, in.Echeance); err != nil {
				erreur(w, http.StatusBadRequest, "échéance invalide (AAAA-MM-JJ)")
				return false
			}
		}
		return true
	}
	return false
}

// listTaches renvoie les tâches à faire et celles cochées aujourd'hui.
func (s *Server) listTaches(w http.ResponseWriter, r *http.Request) {
	ts, err := s.Store.Taches(r.Context(), userID(r.Context()), s.aujourdhui())
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	res := make([]tacheJSON, 0, len(ts))
	for _, t := range ts {
		res = append(res, versTacheJSON(t))
	}
	ecrireJSON(w, http.StatusOK, res)
}

func (s *Server) createTache(w http.ResponseWriter, r *http.Request) {
	var in tacheJSON
	if !lireJSON(w, r, &in) || !s.validerTache(w, &in) {
		return
	}
	in.Source, in.SourceRef = "", ""
	// Lien d'une capture rapide (Partager › Microdash) : une adresse web seulement.
	if l := strings.TrimSpace(in.Lien); len(l) > 2000 || !(strings.HasPrefix(l, "https://") || strings.HasPrefix(l, "http://")) {
		in.Lien = ""
	} else {
		in.Lien = l
	}
	t, err := s.Store.CreateTache(r.Context(), userID(r.Context()), in.Tache)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusCreated, versTacheJSON(*t))
}

func (s *Server) updateTache(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	var in tacheJSON
	if !lireJSON(w, r, &in) || !s.validerTache(w, &in) {
		return
	}
	ctx, uid := r.Context(), userID(r.Context())
	avant, err := s.Store.Tache(ctx, uid, id)
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "tâche introuvable")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	in.ID = id
	in.FaiteLe = ""
	if in.Faite {
		in.FaiteLe = avant.FaiteLe
		if in.FaiteLe == "" {
			in.FaiteLe = s.aujourdhui().Format(formatDate)
		}
	}
	t, err := s.Store.UpdateTache(ctx, uid, in.Tache)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	if avant.FaiteLe == "" && t.FaiteLe != "" {
		s.finirSource(r, *t)
	}
	ecrireJSON(w, http.StatusOK, versTacheJSON(*t))
}

// finirSource prévient le module d'où vient une tâche qu'elle est faite (l'échéance est
// cochée, le mail archivé). Un échec n'empêche pas de cocher la tâche.
func (s *Server) finirSource(r *http.Request, t store.Tache) {
	if t.Source == "" {
		return
	}
	m, ok := s.Modules.Module(t.Source)
	if !ok {
		return
	}
	if f, ok := m.(modules.Finisseur); ok {
		if err := f.Finir(r.Context(), userID(r.Context()), t.SourceRef); err != nil {
			slog.Error("tâche faite : module d'origine", "module", t.Source, "err", err)
		}
	}
}

func (s *Server) deleteTache(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	err := s.Store.DeleteTache(r.Context(), userID(r.Context()), id)
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "tâche introuvable")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// tacheCeSoir range un élément de « Ce soir » dans les tâches : il quitte la liste et devient
// une tâche, qui garde le lien vers l'élément d'origine.
func (s *Server) tacheCeSoir(w http.ResponseWriter, r *http.Request) {
	in, m, ok := s.lireDecision(w, r)
	if !ok {
		return
	}
	// « Ce soir » : une tâche datée d'aujourd'hui ; « semaine » : la semaine en cours.
	if in.Quand == "" || in.Quand == "ce_soir" {
		in.Quand = taches.Dates
	}
	debut, fin, err := taches.Periode(in.Quand, "", "", s.aujourdhui())
	if err != nil {
		erreur(w, http.StatusBadRequest, err.Error())
		return
	}
	src, ok := m.(modules.Source)
	if !ok || m.Code() == "taches" {
		erreur(w, http.StatusBadRequest, "élément inconnu")
		return
	}
	ctx, uid := r.Context(), userID(r.Context())
	els, err := src.ATraiter(ctx, uid, s.aujourdhui())
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	var el *modules.Element
	for i := range els {
		if els[i].Ref == in.Ref {
			el = &els[i]
		}
	}
	if el == nil {
		erreur(w, http.StatusNotFound, "élément introuvable : il a peut-être déjà été traité")
		return
	}
	t := store.Tache{
		Titre: el.Titre, Notes: el.Detail, Quand: in.Quand, Debut: debut, Fin: fin,
		Source: el.Source, SourceRef: el.Ref, Lien: el.Lien,
	}
	if el.Limite {
		t.Echeance = el.Date
	}
	if utf8.RuneCountInString(t.Titre) > 200 {
		t.Titre = string([]rune(t.Titre)[:199]) + "…"
	}
	cree, err := s.Store.CreateTache(ctx, uid, t)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	if err := s.Store.MarquerATraiter(ctx, uid, m.Code(), in.Ref, nil); err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusCreated, versTacheJSON(*cree))
}

// --- Tâches récurrentes ---

func (s *Server) listTachesRecurrentes(w http.ResponseWriter, r *http.Request) {
	rs, _, err := s.Store.TachesRecurrentes(r.Context(), userID(r.Context()))
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, rs)
}

func (s *Server) createTacheRecurrente(w http.ResponseWriter, r *http.Request) {
	var in store.TacheRecurrente
	if !lireJSON(w, r, &in) {
		return
	}
	in.Titre = strings.TrimSpace(in.Titre)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.Depuis == "" {
		in.Depuis = s.aujourdhui().Format(formatDate)
	}
	_, projetConnu := taches.Projets[in.Projet]
	_, rythmeConnu := taches.Rythmes[in.Rythme]
	_, errDepuis := time.Parse(formatDate, in.Depuis)
	switch {
	case in.Titre == "":
		erreur(w, http.StatusBadRequest, "écris la tâche (ex. : Compta du mois)")
	case utf8.RuneCountInString(in.Titre) > 200:
		erreur(w, http.StatusBadRequest, "tâche trop longue : 200 caractères au plus, le reste va dans les notes")
	case utf8.RuneCountInString(in.Notes) > 4000:
		erreur(w, http.StatusBadRequest, "notes trop longues : 4 000 caractères au plus")
	case in.Projet != "" && !projetConnu:
		erreur(w, http.StatusBadRequest, "projet inconnu : educari, saas, microdash ou perso")
	case in.Duree < 0 || in.Duree > 1440:
		erreur(w, http.StatusBadRequest, "durée entre 1 minute et 24 heures")
	case !rythmeConnu:
		erreur(w, http.StatusBadRequest, "rythme : jour, semaine ou mois")
	case errDepuis != nil:
		erreur(w, http.StatusBadRequest, "première date invalide (AAAA-MM-JJ)")
	default:
		ctx, uid := r.Context(), userID(r.Context())
		cree, err := s.Store.CreateTacheRecurrente(ctx, uid, in)
		if err != nil {
			erreurInterne(w, r, err)
			return
		}
		// Les premières tâches apparaissent tout de suite.
		if err := taches.GenererRecurrentes(ctx, s.Store, uid, s.aujourdhui()); err != nil {
			erreurInterne(w, r, err)
			return
		}
		ecrireJSON(w, http.StatusCreated, cree)
	}
}

// arreterTacheRecurrente supprime le modèle et ses tâches à venir pas encore faites.
func (s *Server) arreterTacheRecurrente(w http.ResponseWriter, r *http.Request) {
	id, ok := idParam(w, r)
	if !ok {
		return
	}
	err := s.Store.ArreterTacheRecurrente(r.Context(), userID(r.Context()), id, s.aujourdhui())
	if errors.Is(err, store.ErrNotFound) {
		erreur(w, http.StatusNotFound, "tâche récurrente introuvable")
		return
	}
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Bilan des semaines ---

// bilanTaches renvoie, semaine par semaine, les tâches faites et celles pas faites à temps.
func (s *Server) bilanTaches(w http.ResponseWriter, r *http.Request) {
	n := 8
	if v := r.URL.Query().Get("semaines"); v != "" {
		var err error
		if n, err = strconv.Atoi(v); err != nil || n < 1 || n > 104 {
			erreur(w, http.StatusBadRequest, "semaines : entre 1 et 104")
			return
		}
	}
	res, err := s.bilan(r.Context(), userID(r.Context()), n)
	if err != nil {
		erreurInterne(w, r, err)
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]any{"semaines": res, "aujourdhui": s.aujourdhui().Format(formatDate)})
}

func (s *Server) bilan(ctx context.Context, uid int64, n int) ([]taches.BilanSemaine, error) {
	aujourdhui := s.aujourdhui()
	lundi := taches.Lundi(aujourdhui)
	ts, err := s.Store.TachesBilan(ctx, uid, lundi.AddDate(0, 0, -7*(n-1)), lundi.AddDate(0, 0, 6))
	if err != nil {
		return nil, err
	}
	return taches.Bilan(ts, aujourdhui, n), nil
}
