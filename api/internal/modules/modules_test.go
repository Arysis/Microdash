package modules

import (
	"context"
	"testing"
	"time"
)

func jour(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestFiltrer(t *testing.T) {
	els := []Element{
		{Source: "finances", Ref: "a"}, {Source: "finances", Ref: "b"},
		{Source: "finances", Ref: "c"}, {Source: "mails", Ref: "a"},
	}
	etats := map[string]Etat{
		"finances/a": {Statut: "fait"},
		"finances/b": {Statut: "reporte", Jusqua: jour("2026-10-27")},
		"finances/c": {Statut: "reporte", Jusqua: jour("2026-10-26")}, // revient aujourd'hui
	}
	got := Filtrer(els, etats, jour("2026-10-26"))
	if len(got) != 2 || got[0].Cle() != "finances/c" || got[1].Cle() != "mails/a" {
		t.Fatalf("Filtrer = %+v", got)
	}
}

func TestTrier(t *testing.T) {
	els := []Element{
		{Titre: "sans date"},
		{Titre: "plus tard", Date: "2026-11-10"},
		{Titre: "urgent", Date: "2026-11-20", Importance: 2},
		{Titre: "bientôt", Date: "2026-10-31"},
		{Titre: "important", Importance: 1},
	}
	Trier(els)
	want := []string{"urgent", "important", "bientôt", "plus tard", "sans date"}
	for i, w := range want {
		if els[i].Titre != w {
			t.Fatalf("ordre %d = %q, attendu %q (%+v)", i, els[i].Titre, w, els)
		}
	}
}

func TestReporter(t *testing.T) {
	cas := []struct{ quand, depuis, attendu string }{
		{"demain", "2026-10-26", "2026-10-27"},
		{"week-end", "2026-10-26", "2026-10-31"}, // lundi → samedi
		{"week-end", "2026-10-30", "2026-10-31"}, // vendredi → le lendemain
		{"week-end", "2026-10-31", "2026-11-07"}, // samedi → samedi suivant
		{"week-end", "2026-11-01", "2026-11-07"}, // dimanche → samedi suivant
		{"semaine", "2026-10-26", "2026-11-02"},  // lundi → lundi suivant
		{"semaine", "2026-11-01", "2026-11-02"},  // dimanche → le lendemain
	}
	for _, c := range cas {
		got, err := Reporter(c.quand, jour(c.depuis))
		if err != nil || got.Format("2006-01-02") != c.attendu {
			t.Errorf("Reporter(%s, %s) = %s, %v ; attendu %s", c.quand, c.depuis, got.Format("2006-01-02"), err, c.attendu)
		}
	}
	if _, err := Reporter("jamais", jour("2026-10-26")); err == nil {
		t.Error("report inconnu accepté")
	}
}

type fauxModule string

func (f fauxModule) Code() string { return string(f) }
func (f fauxModule) Nom() string  { return string(f) }
func (fauxModule) ATraiter(context.Context, int64, time.Time) ([]Element, error) {
	return nil, nil
}

func TestRegistre(t *testing.T) {
	r := NewRegistre(fauxModule("finances"), fauxModule("mails"))
	if m, ok := r.Module("mails"); !ok || m.Code() != "mails" {
		t.Fatal("module mails introuvable")
	}
	if _, ok := r.Module("crm"); ok {
		t.Fatal("module inconnu trouvé")
	}
	if len(r.Modules()) != 2 || r.Modules()[0].Code() != "finances" {
		t.Fatal("ordre des modules perdu")
	}
}
