package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/arysis/microdash/api/internal/modules"
	"github.com/arysis/microdash/api/internal/modules/taches"
	"github.com/arysis/microdash/api/internal/store"
)

// --- Tâches ---

var listesTaches = map[string]bool{"ce_soir": true, "semaine": true, "plus_tard": true}

type tacheJSON struct {
	store.Tache
	Faite bool `json:"faite"`
}

func versTacheJSON(t store.Tache) tacheJSON { return tacheJSON{Tache: t, Faite: t.FaiteLe != ""} }

func (s *Server) validerTache(w http.ResponseWriter, in *tacheJSON) bool {
	in.Titre = strings.TrimSpace(in.Titre)
	in.Notes = strings.TrimSpace(in.Notes)
	if in.Quand == "" {
		in.Quand = "semaine"
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
	case !listesTaches[in.Quand]:
		erreur(w, http.StatusBadRequest, "liste inconnue : ce_soir, semaine ou plus_tard")
	default:
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
	in.Source, in.SourceRef, in.Lien = "", "", ""
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
	if in.Quand == "" {
		in.Quand = "ce_soir"
	}
	src, ok := m.(modules.Source)
	if !ok || m.Code() == "taches" || !listesTaches[in.Quand] {
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
		Titre: el.Titre, Notes: el.Detail, Quand: in.Quand,
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
