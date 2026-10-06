package calc

import "testing"

func TestPaiementURSSAFMensuel(t *testing.T) {
	set := charger(t)
	p := Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2024-03-01"), Periodicite: Mensuelle}
	txs := []Transaction{
		{Type: Recette, Date: jour("2026-09-10"), Centimes: 500000},
		{Type: Recette, Date: jour("2026-10-02"), Centimes: 100000},
		{Type: Depense, Date: jour("2026-10-05"), Centimes: 3000},
		// Payé début octobre pour septembre, un peu moins que l'estimation (1 075 €).
		{Type: Depense, Date: jour("2026-10-03"), Centimes: 106000, Echeance: "urssaf-2026-09"},
		// Décembre payé en janvier suivant.
		{Type: Recette, Date: jour("2026-12-10"), Centimes: 200000},
		{Type: Depense, Date: jour("2027-01-08"), Centimes: 43000, Echeance: "urssaf-2026-12"},
	}
	r, err := Calculer(2026, p, txs, set)
	if err != nil {
		t.Fatal(err)
	}
	sep, oct, dec := r.Mois[8].Totaux, r.Mois[9].Totaux, r.Mois[11].Totaux
	if sep.URSSAFPaye != 106000 || sep.Cotisations+sep.CFP+sep.ImpotVL != 0 || sep.Net != 500000-106000 {
		t.Errorf("septembre = %+v", sep)
	}
	// Le paiement n'est pas une dépense d'octobre ; octobre garde son estimation.
	if oct.Depenses != 3000 || oct.URSSAFPaye != 0 || oct.Cotisations == 0 {
		t.Errorf("octobre = %+v", oct)
	}
	if dec.URSSAFPaye != 43000 || dec.Cotisations != 0 {
		t.Errorf("décembre = %+v", dec)
	}
	if r.Total.URSSAFPaye != 149000 || r.Total.Depenses != 3000 {
		t.Errorf("total = %+v", r.Total)
	}

	ag, err := Agenda(2026, p, Mensuelle, txs, set)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ag {
		switch e.Code {
		case "urssaf-2026-09":
			if e.Paye != 106000 || e.APayer != 0 || e.CA != 500000 {
				t.Errorf("échéance septembre = %+v", e)
			}
		case "urssaf-2026-10":
			if e.Paye != 0 || e.APayer != 21500 {
				t.Errorf("échéance octobre = %+v", e)
			}
		}
	}
}

func TestPaiementURSSAFTrimestriel(t *testing.T) {
	set := charger(t)
	p := Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2024-03-01"), Periodicite: Trimestrielle}
	txs := []Transaction{
		{Type: Recette, Date: jour("2026-07-10"), Centimes: 300000},
		{Type: Recette, Date: jour("2026-08-10"), Centimes: 300000},
		{Type: Recette, Date: jour("2026-09-10"), Centimes: 600000},
		{Type: Depense, Date: jour("2026-10-20"), Centimes: 240000, Echeance: "urssaf-2026-t3"},
	}
	r, err := Calculer(2026, p, txs, set)
	if err != nil {
		t.Fatal(err)
	}
	// Réparti au prorata des charges estimées, donc du chiffre d'affaires ici.
	if r.Mois[6].URSSAFPaye != 60000 || r.Mois[7].URSSAFPaye != 60000 || r.Mois[8].URSSAFPaye != 120000 {
		t.Errorf("répartition = %d %d %d", r.Mois[6].URSSAFPaye, r.Mois[7].URSSAFPaye, r.Mois[8].URSSAFPaye)
	}
	if q := r.Trimestres[2]; q.URSSAFPaye != 240000 || q.Cotisations != 0 || q.Net != 1200000-240000 {
		t.Errorf("T3 = %+v", q)
	}
	if r.Mois[9].Depenses != 0 {
		t.Errorf("octobre = %+v", r.Mois[9])
	}

	// En trésorerie, le paiement compte en octobre, à la place de l'estimation du 2 novembre.
	ag, _ := Agenda(2026, p, Trimestrielle, txs, set)
	tr, err := CalculerTresorerie(EntreeTresorerie{
		Aujourdhui: jour("2026-10-25"), Profil: p, Saisies: txs, Echeances: ag, MoisPasses: 1, MoisFuturs: 1,
	}, set)
	if err != nil {
		t.Fatal(err)
	}
	oct, nov := tr.Mois[1], tr.Mois[2]
	if oct.Cotisations != 240000 || oct.Depenses != 0 || nov.Cotisations != 0 {
		t.Errorf("trésorerie octobre %+v, novembre %+v", oct, nov)
	}
}

func TestPeriodeEcheance(t *testing.T) {
	p := Profil{DebutActivite: jour("2026-02-15"), Periodicite: Mensuelle}
	cas := map[string][2]string{
		"urssaf-2026-05": {"2026-02-15", "2026-05-31"}, // première déclaration : du début à fin mai
		"urssaf-2026-06": {"2026-06-01", "2026-06-30"},
		"urssaf-2026-t3": {"2026-07-01", "2026-09-30"}, // code d'une autre périodicité : lu tel quel
	}
	for code, attendu := range cas {
		d, f, ok := PeriodeEcheance(code, p)
		if !ok || d.Format("2006-01-02") != attendu[0] || f.Format("2006-01-02") != attendu[1] {
			t.Errorf("%s = %v %v %v", code, d, f, ok)
		}
	}
	if _, _, ok := PeriodeEcheance("cfe-2026", p); ok {
		t.Error("cfe-2026 accepté")
	}
}

func TestPaiementURSSAFArrondi(t *testing.T) {
	set := charger(t)
	p := Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2024-03-01"), Periodicite: Trimestrielle}
	txs := []Transaction{
		{Type: Recette, Date: jour("2026-07-10"), Centimes: 100000},
		{Type: Recette, Date: jour("2026-08-10"), Centimes: 100000},
		{Type: Recette, Date: jour("2026-09-10"), Centimes: 100000},
		{Type: Depense, Date: jour("2026-10-20"), Centimes: 100000, Echeance: "urssaf-2026-t3"},
	}
	ag, err := Agenda(2026, p, Trimestrielle, txs, set)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ag {
		if e.Code == "urssaf-2026-t3" && e.Paye != 100000 {
			t.Errorf("payé = %d, attendu 100000", e.Paye)
		}
	}
}
