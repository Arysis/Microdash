package taches

import (
	"sort"
	"time"

	"github.com/arysis/microdash/api/internal/store"
)

// LigneBilan est une tâche dans le bilan d'une semaine.
type LigneBilan struct {
	ID      int64  `json:"id"`
	Titre   string `json:"titre"`
	Projet  string `json:"projet,omitempty"`
	Duree   int    `json:"duree_minutes,omitempty"`
	FaiteLe string `json:"faite_le,omitempty"` // pour une tâche pas faite à temps : le jour où elle a été faite après
	Fin     string `json:"fin,omitempty"`
}

// BilanSemaine est le bilan d'une semaine, du lundi au dimanche.
type BilanSemaine struct {
	Lundi     string         `json:"lundi"`
	Dimanche  string         `json:"dimanche"`
	EnCours   bool           `json:"en_cours"`
	Faites    []LigneBilan   `json:"faites"`
	NonFaites []LigneBilan   `json:"non_faites"` // période finie dans la semaine, pas faite à la fin de la semaine
	Minutes   int            `json:"minutes"`    // durée des tâches faites
	ParProjet map[string]int `json:"par_projet"` // tâches faites par projet ("" : sans projet)
}

// Lundi renvoie le lundi de la semaine d'un jour.
func Lundi(j time.Time) time.Time { return j.AddDate(0, 0, -((int(j.Weekday()) + 6) % 7)) }

// Bilan range les tâches par semaine, de la semaine en cours aux plus anciennes (n semaines).
// Les semaines passées sans rien sont omises. Une tâche faite compte la semaine où elle a été
// cochée ; une tâche pas faite compte la semaine où sa période s'est terminée.
func Bilan(ts []store.Tache, aujourdhui time.Time, n int) []BilanSemaine {
	premier := Lundi(aujourdhui)
	semaines := make([]BilanSemaine, n)
	for i := range semaines {
		l := premier.AddDate(0, 0, -7*i)
		semaines[i] = BilanSemaine{Lundi: l.Format(formatDate), Dimanche: l.AddDate(0, 0, 6).Format(formatDate),
			EnCours: i == 0, Faites: []LigneBilan{}, NonFaites: []LigneBilan{}, ParProjet: map[string]int{}}
	}
	ou := func(jour string) *BilanSemaine {
		for i := range semaines {
			if semaines[i].Lundi <= jour && jour <= semaines[i].Dimanche {
				return &semaines[i]
			}
		}
		return nil
	}
	for _, t := range ts {
		l := LigneBilan{ID: t.ID, Titre: t.Titre, Projet: t.Projet, Duree: t.Duree, Fin: t.Fin}
		if t.FaiteLe != "" {
			if s := ou(t.FaiteLe); s != nil {
				s.Faites = append(s.Faites, l)
				s.Minutes += t.Duree
				s.ParProjet[t.Projet]++
			}
		}
		if t.Fin == "" {
			continue
		}
		if s := ou(t.Fin); s != nil && (t.FaiteLe == "" || t.FaiteLe > s.Dimanche) {
			l.FaiteLe = t.FaiteLe
			s.NonFaites = append(s.NonFaites, l)
		}
	}
	res := []BilanSemaine{}
	for _, s := range semaines {
		if !s.EnCours && len(s.Faites) == 0 && len(s.NonFaites) == 0 {
			continue
		}
		sort.SliceStable(s.NonFaites, func(i, j int) bool { return s.NonFaites[i].Fin < s.NonFaites[j].Fin })
		res = append(res, s)
	}
	return res
}
