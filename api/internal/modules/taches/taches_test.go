package taches

import (
	"context"
	"testing"
	"time"

	"github.com/arysis/microdash/api/internal/store"
)

type fausses struct {
	taches []store.Tache
	finies []int64
}

func (f *fausses) TachesDues(context.Context, int64, time.Time) ([]store.Tache, error) {
	return f.taches, nil
}
func (f *fausses) TachesDatees(context.Context, int64, time.Time, time.Time) ([]store.Tache, error) {
	return f.taches, nil
}
func (f *fausses) FinirTache(_ context.Context, _ int64, id int64, _ time.Time) error {
	f.finies = append(f.finies, id)
	return nil
}

func TestATraiter(t *testing.T) {
	f := &fausses{taches: []store.Tache{
		{ID: 1, Titre: "Relancer Paul", Projet: "educari", Duree: 15, Quand: "semaine", Echeance: "2026-10-25"},
		{ID: 2, Titre: "Facture", Quand: "plus_tard", Echeance: "2026-10-26"},
		{ID: 3, Titre: "Déjà ce soir", Quand: "ce_soir", Echeance: "2026-10-20"},
	}}
	aujourdhui := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	els, err := Module{Donnees: f}.ATraiter(context.Background(), 1, aujourdhui)
	if err != nil {
		t.Fatal(err)
	}
	if len(els) != 2 {
		t.Fatalf("%d éléments, attendu 2 (la tâche de ce soir est dans « Ta soirée »)", len(els))
	}
	if els[0].Ref != "1" || els[0].Importance != 2 || els[0].Detail != "Educari · 15 min" {
		t.Errorf("tâche en retard : %+v", els[0])
	}
	if els[1].Importance != 1 || els[1].Detail != "" || !els[1].Finir {
		t.Errorf("tâche du jour : %+v", els[1])
	}
}

func TestFinir(t *testing.T) {
	f := &fausses{}
	m := Module{Donnees: f, Aujourdhui: func() time.Time { return time.Time{} }}
	if err := m.Finir(context.Background(), 1, "42"); err != nil {
		t.Fatal(err)
	}
	if err := m.Finir(context.Background(), 1, "pas-un-nombre"); err != nil {
		t.Fatal(err)
	}
	if len(f.finies) != 1 || f.finies[0] != 42 {
		t.Errorf("tâches cochées : %v", f.finies)
	}
}

func TestDuree(t *testing.T) {
	for min, attendu := range map[int]string{15: "15 min", 60: "1 h", 90: "1 h 30", 125: "2 h 05"} {
		if d := Duree(min); d != attendu {
			t.Errorf("Duree(%d) = %q, attendu %q", min, d, attendu)
		}
	}
}
