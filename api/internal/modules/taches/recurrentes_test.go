package taches

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/arysis/microdash/api/internal/store"
)

func jour(s string) time.Time {
	t, _ := time.Parse(formatDate, s)
	return t
}

func TestOccurrences(t *testing.T) {
	cas := []struct {
		rythme, depuis, du, au, attendu string
	}{
		{"jour", "2026-11-20", "2026-11-19", "2026-11-22", "2026-11-20 2026-11-21 2026-11-22"},
		{"semaine", "2026-11-23", "2026-11-20", "2026-12-08", "2026-11-23 2026-11-30 2026-12-07"}, // chaque lundi
		{"mois", "2026-01-31", "2026-01-01", "2026-04-30", "2026-01-31 2026-02-28 2026-03-31 2026-04-30"},
		{"mois", "2026-11-01", "2026-11-20", "2027-01-05", "2026-12-01 2027-01-01"},
	}
	for _, c := range cas {
		got, err := Occurrences(store.TacheRecurrente{Rythme: c.rythme, Depuis: c.depuis}, jour(c.du), jour(c.au))
		if err != nil || strings.Join(got, " ") != c.attendu {
			t.Errorf("%s depuis %s : %v %v, attendu %s", c.rythme, c.depuis, got, err, c.attendu)
		}
	}
	if _, err := Occurrences(store.TacheRecurrente{Rythme: "an", Depuis: "2026-01-01"}, jour("2026-01-01"), jour("2026-01-02")); err == nil {
		t.Error("rythme inconnu accepté")
	}
}

type fausseRecurrence struct {
	rs    []store.TacheRecurrente
	crees map[int64][]string
}

func (f *fausseRecurrence) TachesRecurrentes(context.Context, int64) ([]store.TacheRecurrente, []int64, error) {
	comptes := make([]int64, len(f.rs))
	for i := range comptes {
		comptes[i] = 1
	}
	return f.rs, comptes, nil
}

func (f *fausseRecurrence) CreerOccurrences(_ context.Context, _ int64, r store.TacheRecurrente, dates []string, jusqua string) error {
	f.crees[r.ID] = append(f.crees[r.ID], dates...)
	for i := range f.rs {
		if f.rs[i].ID == r.ID {
			f.rs[i].GenereJusqua = jusqua
		}
	}
	return nil
}

func TestGenererRecurrentes(t *testing.T) {
	f := &fausseRecurrence{crees: map[int64][]string{}, rs: []store.TacheRecurrente{
		{ID: 1, Rythme: "jour", Depuis: "2026-11-20"},
		{ID: 2, Rythme: "semaine", Depuis: "2026-11-23"},
	}}
	// Deux passages le même jour, puis le lendemain : chaque date une seule fois.
	for _, j := range []string{"2026-11-20", "2026-11-20", "2026-11-21"} {
		if err := GenererRecurrentes(context.Background(), f, 0, jour(j)); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Join(f.crees[1], " "); got != "2026-11-20 2026-11-21" {
		t.Errorf("chaque jour : %s", got)
	}
	if got := strings.Join(f.crees[2], " "); got != "2026-11-23" {
		t.Errorf("chaque lundi : %s", got)
	}
	// Une semaine plus tard, le lundi suivant arrive.
	_ = GenererRecurrentes(context.Background(), f, 0, jour("2026-11-24"))
	if got := strings.Join(f.crees[2], " "); got != "2026-11-23 2026-11-30" {
		t.Errorf("lundi suivant : %s", got)
	}
}

func TestBilan(t *testing.T) {
	ts := []store.Tache{
		{ID: 1, Titre: "Rig", Projet: "perso", Duree: 60, Debut: "2026-11-16", Fin: "2026-11-16", FaiteLe: "2026-11-16"},
		{ID: 2, Titre: "Feu", Projet: "perso", Debut: "2026-11-17", Fin: "2026-11-17"},                                     // jamais faite
		{ID: 3, Titre: "Compta", Projet: "saas", Duree: 30, Debut: "2026-11-12", Fin: "2026-11-13", FaiteLe: "2026-11-18"}, // faite en retard
		{ID: 4, Titre: "Idée", Quand: "plus_tard", FaiteLe: "2026-11-20"},
		{ID: 5, Titre: "Caméra", Debut: "2026-11-21", Fin: "2026-11-22"}, // semaine en cours, pas encore finie
	}
	bs := Bilan(ts, jour("2026-11-20"), 4)
	// Semaine en cours (16-22), semaine du 9 ; celle du 2 est vide et omise.
	if len(bs) != 2 || bs[0].Lundi != "2026-11-16" || !bs[0].EnCours || bs[1].Lundi != "2026-11-09" {
		t.Fatalf("semaines : %+v", bs)
	}
	s := bs[0]
	if len(s.Faites) != 3 || s.Minutes != 90 || s.ParProjet["perso"] != 1 || s.ParProjet["saas"] != 1 || s.ParProjet[""] != 1 {
		t.Errorf("faites : %+v", s)
	}
	if len(s.NonFaites) != 2 || s.NonFaites[0].Titre != "Feu" || s.NonFaites[1].Titre != "Caméra" {
		t.Errorf("non faites : %+v", s.NonFaites)
	}
	p := bs[1]
	if len(p.Faites) != 0 || len(p.NonFaites) != 1 || p.NonFaites[0].Titre != "Compta" || p.NonFaites[0].FaiteLe != "2026-11-18" {
		t.Errorf("semaine passée : %+v", p)
	}
}
