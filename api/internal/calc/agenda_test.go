package calc

import (
	"testing"
)

func TestPaquesEtFeries(t *testing.T) {
	for annee, attendu := range map[int]string{2024: "2024-03-31", 2026: "2026-04-05", 2027: "2027-03-28"} {
		if got := paques(annee).Format("2006-01-02"); got != attendu {
			t.Errorf("paques(%d) = %s, attendu %s", annee, got, attendu)
		}
	}
	for _, j := range []string{"2026-01-01", "2026-04-06", "2026-05-14", "2026-05-25", "2026-11-01", "2026-12-25"} {
		if !Ferie(jour(j)) {
			t.Errorf("%s devrait être férié", j)
		}
	}
	if Ferie(jour("2026-04-05")) || Ferie(jour("2026-11-02")) {
		t.Error("jour non férié compté comme férié")
	}
}

func TestJourOuvre(t *testing.T) {
	cas := map[string]string{
		"2026-01-31": "2026-02-02", // samedi
		"2026-10-31": "2026-11-02", // samedi, puis dimanche 1er novembre férié
		"2027-01-31": "2027-02-01", // dimanche
		"2026-04-30": "2026-04-30", // jeudi
	}
	for d, attendu := range cas {
		if got := JourOuvre(jour(d)).Format("2006-01-02"); got != attendu {
			t.Errorf("JourOuvre(%s) = %s, attendu %s", d, got, attendu)
		}
	}
}

type attendue struct{ code, date, legale, debut, fin string }

func verifier(t *testing.T, es []Echeance, att []attendue) {
	t.Helper()
	if len(es) != len(att) {
		for _, e := range es {
			t.Logf("%s %s", e.Code, e.Date)
		}
		t.Fatalf("%d échéances, attendu %d", len(es), len(att))
	}
	for i, a := range att {
		e := es[i]
		if e.Code != a.code || e.Date != a.date || e.DateLegale != a.legale ||
			(a.debut != "" && (e.PeriodeDebut != a.debut || e.PeriodeFin != a.fin)) {
			t.Errorf("échéance %d = %+v, attendu %+v", i, e, a)
		}
	}
}

func TestAgendaTrimestriel(t *testing.T) {
	p := Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2024-03-01")}
	txs := []Transaction{
		{Type: Recette, Date: jour("2025-11-10"), Centimes: 100000},
		{Type: Recette, Date: jour("2026-10-01"), Centimes: 180000},
	}
	es, err := Agenda(2026, p, Trimestrielle, txs, charger(t))
	if err != nil {
		t.Fatal(err)
	}
	verifier(t, es, []attendue{
		{"urssaf-2025-t4", "2026-02-02", "2026-01-31", "2025-10-01", "2025-12-31"},
		{"urssaf-2026-t1", "2026-04-30", "", "2026-01-01", "2026-03-31"},
		{"revenus-2025", "", "", "", ""},
		{"urssaf-2026-t2", "2026-07-31", "", "2026-04-01", "2026-06-30"},
		{"urssaf-2026-t3", "2026-11-02", "2026-10-31", "2026-07-01", "2026-09-30"},
		{"cfe-2026", "2026-12-15", "", "", ""},
	})
	// T4 2025 : 1 000 € de recettes, 21,2 % + 0,3 % de CFP
	if es[0].CA != 100000 || es[0].APayer != 21500 || es[0].Note != "" || es[0].BaremeExact {
		t.Errorf("T4 2025 = %+v", es[0])
	}
	if es[1].CA != 0 || es[1].Note == "" {
		t.Errorf("une période sans recette doit porter la note des 0 € : %+v", es[1])
	}
	if es[2].CA != 100000 || es[2].Note == "" {
		t.Errorf("déclaration de revenus = %+v", es[2])
	}
	if es[0].Libelle != "Déclaration URSSAF du 4e trimestre 2025" {
		t.Errorf("libellé = %q", es[0].Libelle)
	}

	// L'année suivante commence par le 4e trimestre 2026, reporté au lundi 1er février 2027.
	es, err = Agenda(2027, p, Trimestrielle, txs, charger(t))
	if err != nil {
		t.Fatal(err)
	}
	if es[0].Code != "urssaf-2026-t4" || es[0].Date != "2027-02-01" || es[0].DateLegale != "2027-01-31" || es[0].CA != 180000 {
		t.Errorf("T4 2026 = %+v", es[0])
	}
	// Pas encore de barème 2027 : montants 2027 et CFE marqués comme estimés, CFE au 15 décembre quand même.
	if !es[0].BaremeExact || es[1].BaremeExact || es[len(es)-1].BaremeExact || es[len(es)-1].Code != "cfe-2027" || es[len(es)-1].Date != "2027-12-15" {
		t.Errorf("agenda 2027 = %+v", es)
	}
}

func TestPremiereDeclarationTrimestrielle(t *testing.T) {
	// Début le 1er février : du 1er février au 30 juin, à déclarer au 31 juillet.
	p := Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2026-02-01")}
	es, err := Agenda(2026, p, Trimestrielle, nil, charger(t))
	if err != nil {
		t.Fatal(err)
	}
	verifier(t, es, []attendue{
		{"urssaf-2026-t2", "2026-07-31", "", "2026-02-01", "2026-06-30"},
		{"urssaf-2026-t3", "2026-11-02", "2026-10-31", "2026-07-01", "2026-09-30"},
	})
	if es[0].Libelle != "Première déclaration URSSAF" || es[1].Libelle == "Première déclaration URSSAF" {
		t.Errorf("libellés = %q, %q", es[0].Libelle, es[1].Libelle)
	}
}

func TestPremiereDeclarationMensuelle(t *testing.T) {
	// Début le 15 février : du 15 février au 31 mai, à déclarer au 30 juin.
	// Pas de CFE ni de déclaration de revenus l'année de création.
	p := Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2026-02-15")}
	es, err := Agenda(2026, p, Mensuelle, nil, charger(t))
	if err != nil {
		t.Fatal(err)
	}
	verifier(t, es, []attendue{
		{"urssaf-2026-05", "2026-06-30", "", "2026-02-15", "2026-05-31"},
		{"urssaf-2026-06", "2026-07-31", "", "2026-06-01", "2026-06-30"},
		{"urssaf-2026-07", "2026-08-31", "", "2026-07-01", "2026-07-31"},
		{"urssaf-2026-08", "2026-09-30", "", "2026-08-01", "2026-08-31"},
		{"urssaf-2026-09", "2026-11-02", "2026-10-31", "2026-09-01", "2026-09-30"},
		{"urssaf-2026-10", "2026-11-30", "", "2026-10-01", "2026-10-31"},
		{"urssaf-2026-11", "2026-12-31", "", "2026-11-01", "2026-11-30"},
	})
	if es[1].Libelle != "Déclaration URSSAF de juin 2026" {
		t.Errorf("libellé = %q", es[1].Libelle)
	}
}

func TestPremiereDeclarationSurDeuxAnnees(t *testing.T) {
	// Début le 10 novembre 2025 : du 10 novembre 2025 au 28 février 2026, à déclarer au 31 mars.
	p := Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2025-11-10")}
	txs := []Transaction{
		{Type: Recette, Date: jour("2025-12-05"), Centimes: 50000},
		{Type: Recette, Date: jour("2026-01-20"), Centimes: 30000},
		{Type: Recette, Date: jour("2026-03-02"), Centimes: 99900}, // mois suivant
	}
	es, err := Agenda(2026, p, Mensuelle, txs, charger(t))
	if err != nil {
		t.Fatal(err)
	}
	e := es[0]
	if e.Code != "urssaf-2026-02" || e.Date != "2026-03-31" || e.PeriodeDebut != "2025-11-10" || e.CA != 80000 {
		t.Errorf("première déclaration = %+v", e)
	}
	// Activité commencée en 2025 : déclaration de revenus 2025 et CFE 2026 attendues.
	var revenus, cfe bool
	for _, e := range es {
		revenus = revenus || e.Code == "revenus-2025"
		cfe = cfe || e.Code == "cfe-2026"
	}
	if !revenus || !cfe {
		t.Errorf("revenus=%v cfe=%v", revenus, cfe)
	}
}

func TestAgendaErreurs(t *testing.T) {
	p := Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2026-01-01")}
	if _, err := Agenda(2026, p, "annuelle", nil, charger(t)); err == nil {
		t.Error("périodicité inconnue acceptée")
	}
	if es, err := Agenda(2025, p, Mensuelle, nil, charger(t)); err != nil || len(es) != 0 {
		t.Errorf("année avant le début : %v, %v", es, err)
	}
}
