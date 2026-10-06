// Package calc calcule le chiffre d'affaires, les cotisations, l'impôt et le revenu net
// d'une micro-entreprise à partir de son profil, de ses transactions et du barème.
package calc

import (
	"fmt"
	"math"
	"time"

	"github.com/arysis/microdash/api/internal/bareme"
)

type Profil struct {
	Categorie            string
	CategorieSecondaire  string // activité mixte, vide sinon
	NatureCFP            string // commercant, artisan, liberal
	DebutActivite        time.Time
	ACRE                 bool
	VersementLiberatoire bool
	Periodicite          string // mensuelle ou trimestrielle ; sert à retrouver la période d'un paiement URSSAF
}

const (
	Recette = "recette"
	Depense = "depense"
)

type Transaction struct {
	Type      string
	Date      time.Time
	Centimes  int64
	Categorie string // recettes seulement ; vide = catégorie principale du profil
	// Echeance : pour une dépense, code de la déclaration URSSAF qu'elle paie (ex. urssaf-2026-09).
	// Ce paiement remplace l'estimation de la période et ne compte pas comme une dépense.
	Echeance string
}

// PosteURSSAF est le poste de dépense des paiements à l'URSSAF (cotisations, CFP, impôt).
const PosteURSSAF = "URSSAF (cotisations et impôt)"

// Totaux sont exprimés en centimes.
type Totaux struct {
	CA          int64 `json:"ca"`
	Cotisations int64 `json:"cotisations"`
	CFP         int64 `json:"cfp"`
	ImpotVL     int64 `json:"impot_vl"`
	// URSSAFPaye : ce qui a vraiment été payé à l'URSSAF pour la période. Les mois payés n'ont
	// plus de cotisations, CFP ni impôt estimés.
	URSSAFPaye int64 `json:"urssaf_paye"`
	Depenses   int64 `json:"depenses"`
	Net        int64 `json:"net"`
}

type Periode struct {
	Libelle string `json:"libelle"`
	Debut   string `json:"debut"`
	Fin     string `json:"fin"`
	Totaux
}

type Plafond struct {
	Code    string  `json:"code"`
	Libelle string  `json:"libelle"`
	CA      int64   `json:"ca"`
	Plafond int64   `json:"plafond"`
	Majore  int64   `json:"majore,omitempty"`
	Ratio   float64 `json:"ratio"`
	Niveau  string  `json:"niveau"` // ok, attention, depasse
}

type Resultat struct {
	Annee           int       `json:"annee"`
	BaremeAnnee     int       `json:"bareme_annee"`
	BaremeExact     bool      `json:"bareme_exact"`
	Mois            []Periode `json:"mois"`
	Trimestres      []Periode `json:"trimestres"`
	Total           Totaux    `json:"total"`
	RevenuImposable int64     `json:"revenu_imposable"` // sans versement libératoire, sinon 0
	FinACRE         string    `json:"fin_acre,omitempty"`
	Plafonds        []Plafond `json:"plafonds"`
}

var moisFR = []string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}

// FinACRE renvoie le dernier jour couvert par l'ACRE : la fin du 3e trimestre civil
// qui suit celui du début d'activité.
func FinACRE(debut time.Time) time.Time {
	trimestre := (int(debut.Month()) - 1) / 3
	premierMoisSuivant := time.Date(debut.Year(), time.Month(trimestre*3+1), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 12, 0)
	return premierMoisSuivant.AddDate(0, 0, -1)
}

// Valider vérifie que le profil n'utilise que des catégories connues du barème.
func Valider(p Profil, b *bareme.Bareme) error {
	if _, ok := b.Categories[p.Categorie]; !ok {
		return fmt.Errorf("catégorie inconnue : %q", p.Categorie)
	}
	if p.CategorieSecondaire != "" {
		if _, ok := b.Categories[p.CategorieSecondaire]; !ok {
			return fmt.Errorf("catégorie secondaire inconnue : %q", p.CategorieSecondaire)
		}
		if p.CategorieSecondaire == p.Categorie {
			return fmt.Errorf("la catégorie secondaire doit différer de la principale")
		}
	}
	if _, ok := b.CFP[p.NatureCFP]; !ok {
		return fmt.Errorf("nature d'activité inconnue : %q", p.NatureCFP)
	}
	return nil
}

type accu struct{ ca, cotis, cfp, vl, dep, paye float64 }

func (a *accu) ajouter(b accu) {
	a.ca += b.ca
	a.cotis += b.cotis
	a.cfp += b.cfp
	a.vl += b.vl
	a.dep += b.dep
	a.paye += b.paye
}

func (a *accu) totaux() Totaux {
	t := Totaux{
		CA:          int64(math.Round(a.ca)),
		Cotisations: int64(math.Round(a.cotis)),
		CFP:         int64(math.Round(a.cfp)),
		ImpotVL:     int64(math.Round(a.vl)),
		URSSAFPaye:  int64(math.Round(a.paye)),
		Depenses:    int64(math.Round(a.dep)),
	}
	t.Net = t.CA - t.Cotisations - t.CFP - t.ImpotVL - t.URSSAFPaye - t.Depenses
	return t
}

// Calculer produit le tableau de bord d'une année civile.
func Calculer(annee int, p Profil, txs []Transaction, set *bareme.Set) (*Resultat, error) {
	b, exact := set.Pour(annee)
	if err := Valider(p, b); err != nil {
		return nil, err
	}
	finACRE := FinACRE(p.DebutActivite)
	est, err := estimer(p, txs, set)
	if err != nil {
		return nil, err
	}
	payes := repartirPaiements(p, txs, est)

	var mois [12]accu
	caParCat := map[string]float64{}
	for i := range mois {
		cle := cleMois(time.Date(annee, time.Month(i+1), 1, 0, 0, 0, 0, time.UTC))
		if e := est[cle]; e != nil {
			mois[i] = e.accu
		}
		if v, ok := payes[cle]; ok {
			mois[i].cotis, mois[i].cfp, mois[i].vl, mois[i].paye = 0, 0, 0, v
		}
	}
	for _, t := range txs {
		if t.Type == Recette && t.Date.Year() == annee {
			code := t.Categorie
			if code == "" {
				code = p.Categorie
			}
			caParCat[code] += float64(t.Centimes)
		}
	}

	r := &Resultat{Annee: annee, BaremeAnnee: b.Annee, BaremeExact: exact}
	if p.ACRE {
		r.FinACRE = finACRE.Format("2006-01-02")
	}
	var total accu
	for i := range mois {
		debut := time.Date(annee, time.Month(i+1), 1, 0, 0, 0, 0, time.UTC)
		r.Mois = append(r.Mois, Periode{
			Libelle: moisFR[i], Debut: debut.Format("2006-01-02"),
			Fin: debut.AddDate(0, 1, -1).Format("2006-01-02"), Totaux: mois[i].totaux(),
		})
		total.ajouter(mois[i])
	}
	for q := 0; q < 4; q++ {
		var a accu
		for i := q * 3; i < q*3+3; i++ {
			a.ajouter(mois[i])
		}
		debut := time.Date(annee, time.Month(q*3+1), 1, 0, 0, 0, 0, time.UTC)
		r.Trimestres = append(r.Trimestres, Periode{
			Libelle: fmt.Sprintf("T%d", q+1), Debut: debut.Format("2006-01-02"),
			Fin: debut.AddDate(0, 3, -1).Format("2006-01-02"), Totaux: a.totaux(),
		})
	}
	r.Total = total.totaux()

	if !p.VersementLiberatoire {
		imposable := 0.0
		for code, ca := range caParCat {
			imposable += ca * (1 - b.Categories[code].Abattement)
		}
		if total.ca > 0 && imposable < b.AbattementMinimum*100 {
			imposable = b.AbattementMinimum * 100
		}
		r.RevenuImposable = int64(math.Round(imposable))
	}

	r.Plafonds = plafonds(annee, p, b, caParCat)
	return r, nil
}

func niveau(ratio float64) string {
	switch {
	case ratio >= 1:
		return "depasse"
	case ratio >= 0.8:
		return "attention"
	}
	return "ok"
}

func nouveauPlafond(code, libelle string, ca float64, plafondEuros, majoreEuros float64) Plafond {
	pl := Plafond{Code: code, Libelle: libelle, CA: int64(math.Round(ca)), Plafond: int64(math.Round(plafondEuros * 100)), Majore: int64(math.Round(majoreEuros * 100))}
	if pl.Plafond > 0 {
		pl.Ratio = ca / float64(pl.Plafond)
	}
	pl.Niveau = niveau(pl.Ratio)
	return pl
}

// prorata réduit un plafond l'année de création, au nombre de jours d'activité.
func prorata(annee int, debut time.Time, montant float64) float64 {
	if debut.Year() != annee {
		return montant
	}
	debutAnnee := time.Date(annee, 1, 1, 0, 0, 0, 0, time.UTC)
	jours := debutAnnee.AddDate(1, 0, 0).Sub(debutAnnee).Hours() / 24
	actifs := debutAnnee.AddDate(1, 0, 0).Sub(debut).Hours() / 24
	return math.Round(montant * actifs / jours)
}

func plafonds(annee int, p Profil, b *bareme.Bareme, caParCat map[string]float64) []Plafond {
	codes := []string{p.Categorie}
	if p.CategorieSecondaire != "" {
		codes = append(codes, p.CategorieSecondaire)
	}
	parGroupe := map[string]float64{}
	exemple := map[string]bareme.Categorie{}
	var ordre []string
	for _, c := range codes {
		cat := b.Categories[c]
		if _, vu := exemple[cat.GroupePlafond]; !vu {
			ordre = append(ordre, cat.GroupePlafond)
			exemple[cat.GroupePlafond] = cat
		}
	}
	totalCA := 0.0
	for code, ca := range caParCat {
		parGroupe[b.Categories[code].GroupePlafond] += ca
		totalCA += ca
	}

	var res []Plafond
	mixte := len(ordre) > 1
	if mixte {
		res = append(res, nouveauPlafond("micro_global", "Plafond micro-entreprise (activité mixte)", totalCA, prorata(annee, p.DebutActivite, b.PlafondMixte), 0))
	}
	for _, g := range ordre {
		cat := exemple[g]
		res = append(res, nouveauPlafond("micro_"+g, "Plafond micro-entreprise ("+g+")", parGroupe[g], prorata(annee, p.DebutActivite, cat.PlafondCA), 0))
	}
	for _, g := range ordre {
		cat := exemple[g]
		res = append(res, nouveauPlafond("tva_"+g, "Franchise en base de TVA ("+g+")", parGroupe[g], cat.TVAFranchise.Base, cat.TVAFranchise.Majore))
	}
	return res
}

// moisEstime est l'estimation d'un mois civil.
type moisEstime struct{ accu }

// estimer calcule, pour chaque mois qui a des saisies, le chiffre d'affaires, les cotisations,
// la CFP et l'impôt estimés, et les dépenses hors paiements URSSAF. Chaque saisie suit le barème
// de son année.
func estimer(p Profil, txs []Transaction, set *bareme.Set) (map[string]*moisEstime, error) {
	finACRE := FinACRE(p.DebutActivite)
	res := map[string]*moisEstime{}
	for _, t := range txs {
		b, _ := set.Pour(t.Date.Year())
		k := cleMois(t.Date)
		m := res[k]
		if m == nil {
			m = &moisEstime{}
			res[k] = m
		}
		montant := float64(t.Centimes)
		switch t.Type {
		case Depense:
			if t.Echeance == "" {
				m.dep += montant
			}
		case Recette:
			code := t.Categorie
			if code == "" {
				code = p.Categorie
			}
			cat, ok := b.Categories[code]
			if !ok {
				return nil, fmt.Errorf("catégorie inconnue : %q", code)
			}
			taux := cat.TauxCotisation(t.Date)
			if p.ACRE && !t.Date.After(finACRE) && !t.Date.Before(p.DebutActivite) {
				taux *= 1 - b.ReductionACRE(p.DebutActivite)
			}
			m.ca += montant
			m.cotis += montant * taux
			m.cfp += montant * b.CFP[p.NatureCFP]
			if p.VersementLiberatoire {
				m.vl += montant * cat.VersementLiberatoire
			}
		default:
			return nil, fmt.Errorf("type de transaction inconnu : %q", t.Type)
		}
	}
	return res, nil
}

// repartirPaiements rattache chaque paiement URSSAF aux mois de la période qu'il paie, au prorata
// des charges estimées de chaque mois (à défaut du chiffre d'affaires, à défaut à parts égales).
// Renvoie le montant payé par mois (AAAA-MM), pour tous les mois des périodes payées.
func repartirPaiements(p Profil, txs []Transaction, est map[string]*moisEstime) map[string]float64 {
	parCode := map[string]int64{}
	var codes []string
	for _, t := range txs {
		if t.Type != Depense || t.Echeance == "" {
			continue
		}
		if _, vu := parCode[t.Echeance]; !vu {
			codes = append(codes, t.Echeance)
		}
		parCode[t.Echeance] += t.Centimes
	}
	res := map[string]float64{}
	for _, code := range codes {
		debut, fin, ok := PeriodeEcheance(code, p)
		if !ok {
			continue
		}
		var cles []string
		var charges, ca []float64
		var totalCharges, totalCA float64
		for m := time.Date(debut.Year(), debut.Month(), 1, 0, 0, 0, 0, time.UTC); !m.After(fin); m = m.AddDate(0, 1, 0) {
			k := cleMois(m)
			var c, v float64
			if e := est[k]; e != nil {
				c, v = e.cotis+e.cfp+e.vl, e.ca
			}
			cles, charges, ca = append(cles, k), append(charges, c), append(ca, v)
			totalCharges += c
			totalCA += v
		}
		poids, total := charges, totalCharges
		if total <= 0 {
			poids, total = ca, totalCA
		}
		// Parts en centimes entiers ; le dernier mois reçoit le reste, pour que la somme tombe juste.
		montant := parCode[code]
		reste := montant
		for i, k := range cles {
			part := reste
			if i < len(cles)-1 {
				part = int64(math.Round(float64(montant) / float64(len(cles))))
				if total > 0 {
					part = int64(math.Round(float64(montant) * poids[i] / total))
				}
			}
			res[k] += float64(part)
			reste -= part
		}
	}
	return res
}

// PeriodeEcheance renvoie la période (premier et dernier jour) que couvre une déclaration URSSAF.
// La première déclaration, qui regroupe plusieurs périodes, est retrouvée dans le calendrier du
// profil ; sinon le code suffit : urssaf-AAAA-MM pour un mois, urssaf-AAAA-tN pour un trimestre.
func PeriodeEcheance(code string, p Profil) (debut, fin time.Time, ok bool) {
	var a, n int
	trimestre := false
	if _, err := fmt.Sscanf(code, "urssaf-%d-t%d", &a, &n); err == nil && n >= 1 && n <= 4 {
		trimestre = true
	} else if _, err := fmt.Sscanf(code, "urssaf-%d-%d", &a, &n); err != nil || n < 1 || n > 12 {
		return time.Time{}, time.Time{}, false
	}
	if !p.DebutActivite.IsZero() && (p.Periodicite == Mensuelle || p.Periodicite == Trimestrielle) {
		for annee := a; annee <= a+1; annee++ {
			for _, per := range declarationsURSSAF(annee, p.DebutActivite, p.Periodicite) {
				if codeURSSAF(per, p.Periodicite) == code {
					return per.debut, per.fin, true
				}
			}
		}
	}
	if trimestre {
		d := date(a, time.Month(n*3-2), 1)
		return d, d.AddDate(0, 3, -1), true
	}
	d := date(a, time.Month(n), 1)
	return d, d.AddDate(0, 1, -1), true
}
