package taches

import (
	"context"
	"strings"
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
		{ID: 3, Titre: "Déjà ce soir", Quand: "dates", Debut: "2026-10-26", Fin: "2026-10-28", Echeance: "2026-10-20"},
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

func TestPeriode(t *testing.T) {
	auj := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC) // mercredi
	cas := []struct {
		quand, debut, fin string
		d, f              string
		err               bool
	}{
		{"semaine", "", "", "2026-11-02", "2026-11-08", false},
		{"semaine", "2026-11-15", "", "2026-11-09", "2026-11-15", false}, // un dimanche : sa semaine
		{"dates", "", "", "2026-11-04", "2026-11-04", false},
		{"dates", "2026-11-05", "2026-11-07", "2026-11-05", "2026-11-07", false},
		{"dates", "2026-11-07", "2026-11-05", "", "", true},
		{"plus_tard", "2026-11-05", "", "", "", false},
		{"ce_soir", "", "", "", "", true},
	}
	for _, c := range cas {
		d, f, err := Periode(c.quand, c.debut, c.fin, auj)
		if (err != nil) != c.err || d != c.d || f != c.f {
			t.Errorf("Periode(%s, %q, %q) = %s, %s, %v", c.quand, c.debut, c.fin, d, f, err)
		}
	}
}

func TestEvenements(t *testing.T) {
	f := &fausses{taches: []store.Tache{
		{ID: 1, Titre: "Bot", Quand: "dates", Debut: "2026-10-30", Fin: "2026-11-03"},
		{ID: 2, Titre: "Dossier", Quand: "plus_tard", Echeance: "2026-11-02"},
		{ID: 3, Titre: "Site", Quand: "semaine", Debut: "2026-11-02", Fin: "2026-11-08", Echeance: "2026-11-04"},
	}}
	du, au := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	evs, err := Module{Donnees: f}.Evenements(context.Background(), 1, du, au)
	if err != nil {
		t.Fatal(err)
	}
	var vus []string
	for _, e := range evs {
		vus = append(vus, e.Ref+" "+e.Date)
	}
	// Bot du 1er au 3 (coupé à la période demandée), Dossier à son échéance, Site les 2 et 3 :
	// son échéance du 4 est hors période et dans sa semaine.
	attendus := []string{"1-2026-11-01 2026-11-01", "1-2026-11-02 2026-11-02", "1-2026-11-03 2026-11-03",
		"2-echeance 2026-11-02", "3-2026-11-02 2026-11-02", "3-2026-11-03 2026-11-03"}
	if strings.Join(vus, ",") != strings.Join(attendus, ",") {
		t.Errorf("événements : %v, attendu %v", vus, attendus)
	}
}
