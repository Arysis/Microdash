package taches

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/arysis/microdash/api/internal/store"
)

// Rythmes des tâches récurrentes.
var Rythmes = map[string]string{"jour": "Chaque jour", "semaine": "Chaque semaine", "mois": "Chaque mois"}

// horizon : jours à l'avance où les tâches sont créées. Chaque jour : le jour même seulement,
// pour ne pas remplir la liste ; sinon une semaine, pour voir la prochaine dans l'agenda.
func horizon(rythme string) int {
	if rythme == "jour" {
		return 0
	}
	return 6
}

// Occurrences renvoie les dates (AAAA-MM-JJ) d'un modèle entre deux jours inclus. Chaque mois :
// le même quantième que la première date, ou le dernier jour du mois s'il est plus court.
func Occurrences(r store.TacheRecurrente, du, au time.Time) ([]string, error) {
	depuis, err := time.Parse(formatDate, r.Depuis)
	if err != nil {
		return nil, err
	}
	if du.Before(depuis) {
		du = depuis
	}
	var res []string
	for d := du; !d.After(au); d = d.AddDate(0, 0, 1) {
		ok := false
		switch r.Rythme {
		case "jour":
			ok = true
		case "semaine":
			ok = d.Weekday() == depuis.Weekday()
		case "mois":
			dernier := time.Date(d.Year(), d.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
			ok = d.Day() == min(depuis.Day(), dernier)
		default:
			return nil, errors.New("rythme inconnu : jour, semaine ou mois")
		}
		if ok {
			res = append(res, d.Format(formatDate))
		}
	}
	return res, nil
}

// DonneesRecurrentes est ce que la création des tâches récurrentes lit et écrit.
type DonneesRecurrentes interface {
	TachesRecurrentes(ctx context.Context, userID int64) ([]store.TacheRecurrente, []int64, error)
	CreerOccurrences(ctx context.Context, userID int64, r store.TacheRecurrente, dates []string, jusqua string) error
}

// GenererRecurrentes crée les tâches à venir de chaque modèle (userID 0 : tous les comptes).
func GenererRecurrentes(ctx context.Context, d DonneesRecurrentes, userID int64, aujourdhui time.Time) error {
	rs, comptes, err := d.TachesRecurrentes(ctx, userID)
	if err != nil {
		return err
	}
	for i, r := range rs {
		du := aujourdhui
		if r.GenereJusqua != "" {
			if g, err := time.Parse(formatDate, r.GenereJusqua); err == nil && !g.Before(du) {
				du = g.AddDate(0, 0, 1)
			}
		}
		au := aujourdhui.AddDate(0, 0, horizon(r.Rythme))
		if du.After(au) {
			continue
		}
		dates, err := Occurrences(r, du, au)
		if err != nil {
			slog.Error("tâches récurrentes", "id", r.ID, "err", err)
			continue
		}
		if err := d.CreerOccurrences(ctx, comptes[i], r, dates, au.Format(formatDate)); err != nil {
			return err
		}
	}
	return nil
}
