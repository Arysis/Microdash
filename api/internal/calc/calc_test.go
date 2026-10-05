package calc

import (
	"testing"
	"time"

	"github.com/arysis/microdash/api/baremes"
	"github.com/arysis/microdash/api/internal/bareme"
)

func jour(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func charger(t *testing.T) *bareme.Set {
	t.Helper()
	set, err := bareme.Load(baremes.FS)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func TestFinACRE(t *testing.T) {
	cas := map[string]string{
		"2026-02-15": "2026-12-31",
		"2026-04-01": "2027-03-31",
		"2026-12-31": "2027-09-30",
	}
	for debut, attendu := range cas {
		if got := FinACRE(jour(debut)).Format("2006-01-02"); got != attendu {
			t.Errorf("FinACRE(%s) = %s, attendu %s", debut, got, attendu)
		}
	}
}

func TestServicesBNCSansOptions(t *testing.T) {
	p := Profil{Categorie: "liberal_ssi", NatureCFP: "liberal", DebutActivite: jour("2024-01-01")}
	txs := []Transaction{
		{Type: Recette, Date: jour("2026-01-10"), Centimes: 100000},
		{Type: Recette, Date: jour("2026-05-10"), Centimes: 200000},
		{Type: Depense, Date: jour("2026-05-12"), Centimes: 5000},
		{Type: Recette, Date: jour("2025-12-31"), Centimes: 999999}, // autre année
	}
	r, err := Calculer(2026, p, txs, charger(t))
	if err != nil {
		t.Fatal(err)
	}
	want := Totaux{CA: 300000, Cotisations: 76800, CFP: 600, Depenses: 5000, Net: 300000 - 76800 - 600 - 5000}
	if r.Total != want {
		t.Errorf("total = %+v, attendu %+v", r.Total, want)
	}
	if r.Mois[4].CA != 200000 || r.Trimestres[1].CA != 200000 {
		t.Errorf("répartition mai/T2 incorrecte : %+v / %+v", r.Mois[4].Totaux, r.Trimestres[1].Totaux)
	}
	// 3000 € × (1 − 34 %) = 1980 €
	if r.RevenuImposable != 198000 {
		t.Errorf("revenu imposable = %d, attendu 198000", r.RevenuImposable)
	}
	if !r.BaremeExact || r.BaremeAnnee != 2026 {
		t.Errorf("barème 2026 attendu, obtenu %d exact=%v", r.BaremeAnnee, r.BaremeExact)
	}
}

func TestVersementLiberatoireEtACRE(t *testing.T) {
	p := Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2026-02-01"), ACRE: true, VersementLiberatoire: true}
	txs := []Transaction{
		{Type: Recette, Date: jour("2026-03-01"), Centimes: 100000}, // couvert par l'ACRE (fin 31/12/2026)
	}
	r, err := Calculer(2026, p, txs, charger(t))
	if err != nil {
		t.Fatal(err)
	}
	// 21,2 % réduit de 50 % = 10,6 % ; VL 1,7 % ; CFP artisan 0,3 %
	if r.Total.Cotisations != 10600 || r.Total.ImpotVL != 1700 || r.Total.CFP != 300 {
		t.Errorf("totaux = %+v", r.Total)
	}
	if r.RevenuImposable != 0 {
		t.Errorf("pas de revenu imposable affiché avec le versement libératoire")
	}
	if r.FinACRE != "2026-12-31" {
		t.Errorf("fin ACRE = %s", r.FinACRE)
	}
}

func TestACREReduiteApresJuillet2026(t *testing.T) {
	p := Profil{Categorie: "vente_bic", NatureCFP: "commercant", DebutActivite: jour("2026-07-15"), ACRE: true}
	txs := []Transaction{{Type: Recette, Date: jour("2026-08-01"), Centimes: 100000}}
	r, err := Calculer(2026, p, txs, charger(t))
	if err != nil {
		t.Fatal(err)
	}
	// 12,3 % réduit de 25 % = 9,225 %
	if r.Total.Cotisations != 9225 {
		t.Errorf("cotisations = %d, attendu 9225", r.Total.Cotisations)
	}
}

func TestAbattementMinimum(t *testing.T) {
	p := Profil{Categorie: "vente_bic", NatureCFP: "commercant", DebutActivite: jour("2020-01-01")}
	r, err := Calculer(2026, p, []Transaction{{Type: Recette, Date: jour("2026-03-01"), Centimes: 50000}}, charger(t))
	if err != nil {
		t.Fatal(err)
	}
	if r.RevenuImposable != 30500 {
		t.Errorf("revenu imposable = %d, attendu le minimum de 305 €", r.RevenuImposable)
	}
}

func TestPlafondsMixteEtProrata(t *testing.T) {
	p := Profil{Categorie: "vente_bic", CategorieSecondaire: "services_bic", NatureCFP: "commercant", DebutActivite: jour("2026-07-02")}
	txs := []Transaction{
		{Type: Recette, Date: jour("2026-08-01"), Centimes: 3500000},
		{Type: Recette, Date: jour("2026-08-02"), Centimes: 3200000, Categorie: "services_bic"},
	}
	r, err := Calculer(2026, p, txs, charger(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Plafonds) != 5 {
		t.Fatalf("5 plafonds attendus, obtenu %d : %+v", len(r.Plafonds), r.Plafonds)
	}
	services := r.Plafonds[2]
	// 83 600 € × 183/365 ≈ 41 915 € ; 32 000 € = 76 % → ok
	if services.Code != "micro_services" || services.Plafond != 4191500 || services.Niveau != "ok" {
		t.Errorf("plafond services = %+v", services)
	}
	tva := r.Plafonds[4]
	// 32 000 € / 37 500 € = 85 % → attention
	if tva.Code != "tva_services" || tva.Niveau != "attention" || tva.Majore != 4125000 {
		t.Errorf("franchise TVA services = %+v", tva)
	}
}

func TestBaremeAnneeManquante(t *testing.T) {
	p := Profil{Categorie: "vente_bic", NatureCFP: "commercant", DebutActivite: jour("2020-01-01")}
	r, err := Calculer(2030, p, nil, charger(t))
	if err != nil {
		t.Fatal(err)
	}
	if r.BaremeExact || r.BaremeAnnee != 2026 {
		t.Errorf("le barème 2026 doit servir de repli, obtenu %d exact=%v", r.BaremeAnnee, r.BaremeExact)
	}
}

func TestProfilInvalide(t *testing.T) {
	p := Profil{Categorie: "inconnue", NatureCFP: "liberal"}
	if _, err := Calculer(2026, p, nil, charger(t)); err == nil {
		t.Error("une catégorie inconnue doit être refusée")
	}
}
