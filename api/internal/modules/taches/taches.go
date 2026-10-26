// Package taches est le module Tâches du copilote : il met dans « Ce soir » les tâches dont
// l'échéance est arrivée, et place les tâches datées dans l'agenda.
package taches

import (
	"context"
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

// ATraiter renvoie les tâches arrivées à échéance. Celles rangées dans « Ce soir » n'y sont pas :
// la page les montre déjà dans « Ta soirée ».
func (m Module) ATraiter(ctx context.Context, uid int64, aujourdhui time.Time) ([]modules.Element, error) {
	ts, err := m.Donnees.TachesDues(ctx, uid, aujourdhui)
	if err != nil {
		return nil, err
	}
	res := []modules.Element{}
	jour := aujourdhui.Format(formatDate)
	for _, t := range ts {
		if t.Quand == "ce_soir" {
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

// Evenements place les tâches datées dans l'agenda, le jour de leur échéance.
func (m Module) Evenements(ctx context.Context, uid int64, du, au time.Time) ([]modules.Evenement, error) {
	ts, err := m.Donnees.TachesDatees(ctx, uid, du, au)
	if err != nil {
		return nil, err
	}
	res := make([]modules.Evenement, 0, len(ts))
	for _, t := range ts {
		res = append(res, modules.Evenement{
			Source: codeSource, Ref: strconv.FormatInt(t.ID, 10), Titre: t.Titre, Date: t.Echeance,
			Type: "tache", Fait: t.FaiteLe != "", Lien: lien,
		})
	}
	return res, nil
}
