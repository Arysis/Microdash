// Package agenda est le module Agenda du copilote : les rendez-vous saisis dans l'app.
// La page Agenda et le flux iCal montrent aussi ce que les autres modules y placent.
package agenda

import (
	"context"
	"strconv"
	"time"

	"github.com/arysis/microdash/api/internal/modules"
	"github.com/arysis/microdash/api/internal/store"
)

const codeSource = "agenda"

// Donnees est ce que le module lit en base.
type Donnees interface {
	Evenements(ctx context.Context, userID int64, du, au time.Time) ([]store.Evenement, error)
}

type Module struct{ Donnees Donnees }

func (Module) Code() string { return codeSource }
func (Module) Nom() string  { return "Agenda" }

func (m Module) Evenements(ctx context.Context, uid int64, du, au time.Time) ([]modules.Evenement, error) {
	evs, err := m.Donnees.Evenements(ctx, uid, du, au)
	if err != nil {
		return nil, err
	}
	res := make([]modules.Evenement, 0, len(evs))
	for _, e := range evs {
		res = append(res, modules.Evenement{
			Source: codeSource, Ref: strconv.FormatInt(e.ID, 10), Titre: e.Titre, Date: e.Jour,
			Heure: e.Heure, Duree: e.Duree, Notes: e.Notes, Type: "evenement",
		})
	}
	return res, nil
}
