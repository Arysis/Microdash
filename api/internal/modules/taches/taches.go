// Package taches est le module Tâches du copilote : il met dans « Ce soir » les tâches dont
// l'échéance est arrivée, et place dans l'agenda chaque soir de la période d'une tâche
// (dates ou semaine) et son échéance.
package taches

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/arysis/microdash/api/internal/modules"
	"github.com/arysis/microdash/api/internal/store"
)

const (
	codeSource = "taches"
	formatDate = "2006-01-02"
	lien       = "/taches"
)

// Projets : nom affiché de chaque projet de tâches.
var Projets = map[string]string{
	"educari":   "Educari",
	"saas":      "SaaS",
	"microdash": "Microdash",
	"perso":     "Perso",
}

// Donnees est ce que le module lit et écrit en base.
type Donnees interface {
	TachesDues(ctx context.Context, userID int64, jour time.Time) ([]store.Tache, error)
	TachesDatees(ctx context.Context, userID int64, du, au time.Time) ([]store.Tache, error)
	FinirTache(ctx context.Context, userID, id int64, jour time.Time) error
}

type Module struct {
	Donnees    Donnees
	Aujourdhui func() time.Time // date du jour à Paris, à minuit UTC
}

func (Module) Code() string { return codeSource }
func (Module) Nom() string  { return "Tâches" }

// Listes de tâches : à faire chaque soir de deux dates, d'une semaine, ou un jour plus tard.
const (
	Dates    = "dates"
	Semaine  = "semaine"
	PlusTard = "plus_tard"
)

// Periode renvoie le début et la fin (AAAA-MM-JJ) d'une tâche de la liste donnée : deux dates
// (fin vide : un seul soir), la semaine du lundi au dimanche qui contient debut, ou rien
// pour « Plus tard ». debut vide : aujourd'hui.
func Periode(quand, debut, fin string, aujourdhui time.Time) (string, string, error) {
	if quand == PlusTard {
		return "", "", nil
	}
	d := aujourdhui
	if debut != "" {
		var err error
		if d, err = time.Parse(formatDate, debut); err != nil {
			return "", "", errors.New("date de début invalide (AAAA-MM-JJ)")
		}
	}
	switch quand {
	case Semaine:
		lundi := d.AddDate(0, 0, -((int(d.Weekday()) + 6) % 7))
		return lundi.Format(formatDate), lundi.AddDate(0, 0, 6).Format(formatDate), nil
	case Dates:
		f := d
		if fin != "" {
			var err error
			if f, err = time.Parse(formatDate, fin); err != nil {
				return "", "", errors.New("date de fin invalide (AAAA-MM-JJ)")
			}
		}
		switch {
		case f.Before(d):
			return "", "", errors.New("la date de fin est avant la date de début")
		case f.Sub(d) > 366*24*time.Hour:
			return "", "", errors.New("une tâche dure un an au plus")
		}
		return d.Format(formatDate), f.Format(formatDate), nil
	}
	return "", "", errors.New("liste inconnue : dates, semaine ou plus_tard")
}

// EnCours dit si la tâche est à faire le soir du jour donné (AAAA-MM-JJ).
func EnCours(t store.Tache, jour string) bool {
	return t.Debut != "" && t.Debut <= jour && jour <= t.Fin
}

// ATraiter renvoie les tâches arrivées à échéance. Celles prévues ce soir n'y sont pas :
// la page les montre déjà dans « Ta soirée ».
func (m Module) ATraiter(ctx context.Context, uid int64, aujourdhui time.Time) ([]modules.Element, error) {
	ts, err := m.Donnees.TachesDues(ctx, uid, aujourdhui)
	if err != nil {
		return nil, err
	}
	res := []modules.Element{}
	jour := aujourdhui.Format(formatDate)
	for _, t := range ts {
		if EnCours(t, jour) {
			continue
		}
		el := modules.Element{
			Source: codeSource, Ref: strconv.FormatInt(t.ID, 10), Titre: t.Titre, Date: t.Echeance, Limite: true,
			Detail: detail(t), Importance: 1, Lien: lien, Finir: true,
		}
		if t.Echeance < jour {
			el.Importance = 2
		}
		res = append(res, el)
	}
	return res, nil
}

// detail résume le projet et la durée : « Educari · 15 min ».
func detail(t store.Tache) string {
	d := Projets[t.Projet]
	if t.Duree > 0 {
		if d != "" {
			d += " · "
		}
		d += Duree(t.Duree)
	}
	return d
}

// Duree écrit une durée en minutes : « 15 min », « 1 h », « 1 h 30 ».
func Duree(min int) string {
	h, m := min/60, min%60
	switch {
	case h == 0:
		return strconv.Itoa(m) + " min"
	case m == 0:
		return strconv.Itoa(h) + " h"
	case m < 10:
		return strconv.Itoa(h) + " h 0" + strconv.Itoa(m)
	}
	return strconv.Itoa(h) + " h " + strconv.Itoa(m)
}

// Finir coche la tâche.
func (m Module) Finir(ctx context.Context, uid int64, ref string) error {
	id, err := strconv.ParseInt(ref, 10, 64)
	if err != nil {
		return nil // référence d'une autre version : rien à cocher
	}
	return m.Donnees.FinirTache(ctx, uid, id, m.Aujourdhui())
}

// Evenements place les tâches dans l'agenda : chaque jour de leur période, et le jour de leur
// échéance s'il tombe en dehors.
func (m Module) Evenements(ctx context.Context, uid int64, du, au time.Time) ([]modules.Evenement, error) {
	ts, err := m.Donnees.TachesDatees(ctx, uid, du, au)
	if err != nil {
		return nil, err
	}
	premier, dernier := du.Format(formatDate), au.Format(formatDate)
	res := []modules.Evenement{}
	for _, t := range ts {
		id := strconv.FormatInt(t.ID, 10)
		ev := modules.Evenement{Source: codeSource, Titre: t.Titre, Type: "tache", Fait: t.FaiteLe != "", Lien: lien}
		if t.Debut != "" {
			debut, err1 := time.Parse(formatDate, max(t.Debut, premier))
			fin, err2 := time.Parse(formatDate, min(t.Fin, dernier))
			if err1 != nil || err2 != nil {
				continue
			}
			for d := debut; !d.After(fin); d = d.AddDate(0, 0, 1) {
				ev.Date = d.Format(formatDate)
				ev.Ref = id + "-" + ev.Date
				res = append(res, ev)
			}
		}
		if t.Echeance != "" && t.Echeance >= premier && t.Echeance <= dernier && !EnCours(t, t.Echeance) {
			ev.Date, ev.Ref, ev.Titre = t.Echeance, id+"-echeance", "À rendre : "+t.Titre
			res = append(res, ev)
		}
	}
	return res, nil
}
