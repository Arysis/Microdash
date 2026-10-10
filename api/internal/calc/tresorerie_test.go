package calc

import "testing"

func TestTresorerie(t *testing.T) {
	set := charger(t)
	p := Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2024-03-01")}
	saisies := []Transaction{
		{Type: Recette, Date: jour("2026-07-10"), Centimes: 300000},
		{Type: Recette, Date: jour("2026-08-10"), Centimes: 300000},
		{Type: Recette, Date: jour("2026-09-10"), Centimes: 600000},
		{Type: Depense, Date: jour("2026-09-12"), Centimes: 10000},
		{Type: Recette, Date: jour("2026-10-02"), Centimes: 100000},
		{Type: Recette, Date: jour("2026-10-20"), Centimes: 999999}, // après aujourd'hui : ignorée
	}
	ag, err := Agenda(2026, p, Trimestrielle, saisies[:5], set)
	if err != nil {
		t.Fatal(err)
	}
	ag2, err := Agenda(2027, p, Trimestrielle, saisies[:5], set)
	if err != nil {
		t.Fatal(err)
	}
	solde := int64(500000)
	e := EntreeTresorerie{
		Aujourdhui: jour("2026-10-06"), Profil: p, Saisies: saisies, Echeances: append(ag, ag2...),
		Recurrentes: []Recurrente{{Centimes: 2000, Frequence: Mensuelle, Debut: jour("2026-01-15")}},
		Solde:       &solde, SoldeAu: jour("2026-10-06"), MoisPasses: 3, MoisFuturs: 3,
	}
	tr, err := CalculerTresorerie(e, set)
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Mois) != 7 || tr.Mois[0].Mois != "2026-07" || tr.Mois[6].Mois != "2027-01" {
		t.Fatalf("mois = %+v", tr.Mois)
	}
	// Moyenne de juillet à septembre : 4 000 €.
	if tr.Methode != PrevisionMoyenne || tr.RecettesPrevues != 400000 {
		t.Errorf("prévision = %s %d", tr.Methode, tr.RecettesPrevues)
	}
	// Taux services BIC 21,2 % + CFP artisan 0,3 % = 21,5 %.
	if tr.TauxCharges < 0.2149 || tr.TauxCharges > 0.2151 {
		t.Errorf("taux = %f", tr.TauxCharges)
	}
	sep, oct, nov, dec := tr.Mois[2], tr.Mois[3], tr.Mois[4], tr.Mois[5]
	if sep.Encaisse != 600000 || sep.Depenses != 10000 || sep.Cotisations != 0 || sep.Prevision {
		t.Errorf("septembre = %+v", sep)
	}
	// Octobre compte aussi la dépense récurrente du 15, encore à venir : le solde noté le 6 ne la contient pas.
	if !oct.EnCours || oct.Encaisse != 100000 || oct.Depenses != 2000 || oct.Cotisations != 0 || oct.SoldeFin == nil || *oct.SoldeFin != 498000 {
		t.Errorf("octobre = %+v, solde %d", oct, *oct.SoldeFin)
	}
	// 2 novembre : T3 = 12 000 € × 21,5 % = 2 580 €.
	if !nov.Prevision || nov.Encaisse != 400000 || nov.Depenses != 2000 || nov.Cotisations != 258000 {
		t.Errorf("novembre = %+v", nov)
	}
	if nov.Flux != 400000-2000-258000 || *nov.SoldeFin != 498000+nov.Flux {
		t.Errorf("flux novembre = %d, solde %d", nov.Flux, *nov.SoldeFin)
	}
	if dec.Cotisations != 0 || dec.Depenses != 2000 {
		t.Errorf("décembre = %+v", dec)
	}
	// 1er février 2027 hors horizon : janvier sans cotisations.
	if tr.Mois[6].Cotisations != 0 {
		t.Errorf("janvier = %+v", tr.Mois[6])
	}
	if *tr.SoldePrevu != *tr.Mois[6].SoldeFin {
		t.Errorf("solde prévu = %d", *tr.SoldePrevu)
	}

	// Solde noté le 1er octobre : la recette du 2 s'y ajoute.
	e.SoldeAu = jour("2026-10-01")
	tr, _ = CalculerTresorerie(e, set)
	if v := *tr.Mois[3].SoldeFin; v != 500000+100000-2000 {
		t.Errorf("solde fin octobre noté le 1er = %d", v)
	}
	e.SoldeAu = jour("2026-10-06")

	// Avec un MRR Stripe, il remplace la moyenne.
	mrr := int64(250000)
	e.MRR = &mrr
	tr, _ = CalculerTresorerie(e, set)
	if tr.Methode != PrevisionMRR || tr.Mois[4].Encaisse != 250000 {
		t.Errorf("MRR : %s %+v", tr.Methode, tr.Mois[4])
	}
}

func TestTresorerieT4Prevu(t *testing.T) {
	set := charger(t)
	p := Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2024-03-01")}
	saisies := []Transaction{{Type: Recette, Date: jour("2026-10-02"), Centimes: 100000}}
	ag, _ := Agenda(2026, p, Trimestrielle, saisies, set)
	ag2, _ := Agenda(2027, p, Trimestrielle, saisies, set)
	mrr := int64(100000)
	tr, err := CalculerTresorerie(EntreeTresorerie{Aujourdhui: jour("2026-10-06"), Profil: p, Saisies: saisies, Echeances: append(ag, ag2...), MRR: &mrr, MoisPasses: 0, MoisFuturs: 4}, set)
	if err != nil {
		t.Fatal(err)
	}
	// Février 2027 : T4 = 1 000 € saisis en octobre + novembre et décembre prévus à 1 000 € = 3 000 € × 21,5 %.
	fev := tr.Mois[4]
	if fev.Mois != "2027-02" || fev.Cotisations != 64500 {
		t.Errorf("février = %+v", fev)
	}
}

func TestTresorerieMoyenneActiviteRecente(t *testing.T) {
	set := charger(t)
	saisies := []Transaction{
		{Type: Recette, Date: jour("2026-08-20"), Centimes: 100000}, // août commencé en cours de route
		{Type: Recette, Date: jour("2026-09-10"), Centimes: 300000},
	}
	e := EntreeTresorerie{Aujourdhui: jour("2026-10-06"), Saisies: saisies, MoisPasses: 2, MoisFuturs: 1,
		Profil: Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2026-08-15")}}
	tr, err := CalculerTresorerie(e, set)
	if err != nil {
		t.Fatal(err)
	}
	if tr.MoisMoyenne != 1 || tr.RecettesPrevues != 300000 {
		t.Errorf("moyenne sur %d mois = %d", tr.MoisMoyenne, tr.RecettesPrevues)
	}
	e.Profil.DebutActivite = jour("2026-10-01")
	if tr, _ = CalculerTresorerie(e, set); tr.MoisMoyenne != 0 || tr.RecettesPrevues != 0 {
		t.Errorf("sans mois complet : %d mois, %d", tr.MoisMoyenne, tr.RecettesPrevues)
	}
}

func TestTresorerieSoldeAvant(t *testing.T) {
	set := charger(t)
	solde := int64(100000)
	e := EntreeTresorerie{
		Aujourdhui: jour("2026-10-06"), MoisPasses: 2, MoisFuturs: 0, Solde: &solde, SoldeAu: jour("2026-10-06"),
		Profil: Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2026-08-01")},
		Saisies: []Transaction{
			{Type: Recette, Date: jour("2026-08-10"), Centimes: 30000},
			{Type: Recette, Date: jour("2026-09-10"), Centimes: 50000},
			{Type: Depense, Date: jour("2026-10-02"), Centimes: 10000},
		},
	}
	tr, err := CalculerTresorerie(e, set)
	if err != nil {
		t.Fatal(err)
	}
	// Fin octobre 1 000 € ; fin septembre = 1 000 + 100 de dépense d'octobre ; fin août = 1 100 − 500.
	attendus := []int64{60000, 110000, 100000}
	for i, a := range attendus {
		if m := tr.Mois[i]; m.SoldeFin == nil || *m.SoldeFin != a {
			t.Errorf("%s : %v, attendu %d", m.Mois, m.SoldeFin, a)
		}
	}
}

// Avec un compte connecté, le solde part du solde réel du compte puis suit toutes les saisies
// (même payées ailleurs) et les opérations pas encore validées ; une opération ignorée ne compte pas.
func TestTresorerieBanque(t *testing.T) {
	set := charger(t)
	depart, solde := int64(0), int64(999999) // le solde noté n'est plus utilisé
	e := EntreeTresorerie{
		Aujourdhui: jour("2026-10-09"), MoisPasses: 11, MoisFuturs: 0, Solde: &solde, SoldeAu: jour("2026-10-09"),
		Profil:      Profil{Categorie: "vente_bic", NatureCFP: "commercant", DebutActivite: jour("2026-01-01")},
		SoldeDepart: &depart, BanqueDepuis: jour("2026-01-01"),
		Saisies: []Transaction{
			{Type: Depense, Date: jour("2026-03-05"), Centimes: 2160}, // payée avec une autre carte
			{Type: Recette, Date: jour("2026-06-06"), Centimes: 10000},
			{Type: Depense, Date: jour("2026-07-02"), Centimes: 2800, Echeance: "urssaf-2026-06"},
		},
		Banque: []MouvementBanque{
			{Date: jour("2026-08-01"), Centimes: 1235},                            // à valider
			{Date: jour("2026-10-02"), Centimes: 300, Echeance: "urssaf-2026-09"}, // URSSAF à valider
		},
	}
	tr, err := CalculerTresorerie(e, set)
	if err != nil {
		t.Fatal(err)
	}
	fin := func(mois string) int64 {
		for _, m := range tr.Mois {
			if m.Mois == mois && m.SoldeFin != nil {
				return *m.SoldeFin
			}
		}
		t.Fatalf("%s : pas de solde", mois)
		return 0
	}
	attendus := map[string]int64{"2025-12": 0, "2026-02": 0, "2026-03": -2160, "2026-06": 7840, "2026-07": 5040, "2026-08": 3805}
	for m, a := range attendus {
		if v := fin(m); v != a {
			t.Errorf("fin %s : %d, attendu %d", m, v, a)
		}
	}
	if oct := tr.Mois[len(tr.Mois)-1]; oct.Cotisations != 300 {
		t.Errorf("octobre : cotisations %d, attendu 300 (paiement à valider, sans l'estimation en plus)", oct.Cotisations)
	}
}
