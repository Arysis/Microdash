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
}

// Totaux sont exprimés en centimes.
type Totaux struct {
	CA          int64 `json:"ca"`
	Cotisations int64 `json:"cotisations"`
	CFP         int64 `json:"cfp"`
	ImpotVL     int64 `json:"impot_vl"`
	Depenses    int64 `json:"depenses"`
	Net         int64 `json:"net"`
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

type accu struct{ ca, cotis, cfp, vl, dep float64 }

func (a *accu) totaux() Totaux {
	t := Totaux{
		CA:          int64(math.Round(a.ca)),
		Cotisations: int64(math.Round(a.cotis)),
		CFP:         int64(math.Round(a.cfp)),
		ImpotVL:     int64(math.Round(a.vl)),
		Depenses:    int64(math.Round(a.dep)),
	}
	t.Net = t.CA - t.Cotisations - t.CFP - t.ImpotVL - t.Depenses
	return t
}

// Calculer produit le tableau de bord d'une année civile.
func Calculer(annee int, p Profil, txs []Transaction, set *bareme.Set) (*Resultat, error) {
	b, exact := set.Pour(annee)
	if err := Valider(p, b); err != nil {
		return nil, err
	}
	finACRE := FinACRE(p.DebutActivite)
	reduction := 0.0
	if p.ACRE {
		reduction = b.ReductionACRE(p.DebutActivite)
	}
	tauxCFP := b.CFP[p.NatureCFP]

	var mois [12]accu
	caParCat := map[string]float64{}
	for _, t := range txs {
		if t.Date.Year() != annee {
			continue
		}
		m := &mois[t.Date.Month()-1]
		montant := float64(t.Centimes)
		switch t.Type {
		case Depense:
			m.dep += montant
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
				taux *= 1 - reduction
			}
			m.ca += montant
			m.cotis += montant * taux
			m.cfp += montant * tauxCFP
			if p.VersementLiberatoire {
				m.vl += montant * cat.VersementLiberatoire
			}
			caParCat[code] += montant
		default:
			return nil, fmt.Errorf("type de transaction inconnu : %q", t.Type)
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
		total.ca += mois[i].ca
		total.cotis += mois[i].cotis
		total.cfp += mois[i].cfp
		total.vl += mois[i].vl
		total.dep += mois[i].dep
	}
	for q := 0; q < 4; q++ {
		var a accu
		for i := q * 3; i < q*3+3; i++ {
			a.ca += mois[i].ca
			a.cotis += mois[i].cotis
			a.cfp += mois[i].cfp
			a.vl += mois[i].vl
			a.dep += mois[i].dep
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
