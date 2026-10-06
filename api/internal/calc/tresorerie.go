package calc

import (
	"fmt"
	"math"
	"time"

	"github.com/arysis/microdash/api/internal/bareme"
)

// MoisTresorerie est l'argent qui entre et sort sur un mois, en centimes.
// Les cotisations comptent le mois de leur date limite URSSAF, quand elles sortent du compte.
type MoisTresorerie struct {
	Mois        string `json:"mois"` // AAAA-MM
	Libelle     string `json:"libelle"`
	Encaisse    int64  `json:"encaisse"`
	Depenses    int64  `json:"depenses"`
	Cotisations int64  `json:"cotisations"`
	Flux        int64  `json:"flux"` // encaissé − dépenses − cotisations
	EnCours     bool   `json:"en_cours,omitempty"`
	Prevision   bool   `json:"prevision,omitempty"`
	SoldeFin    *int64 `json:"solde_fin,omitempty"`
}

// MethodePrevision dit d'où viennent les recettes prévues.
const (
	PrevisionMRR     = "mrr"     // revenu mensuel récurrent Stripe
	PrevisionMoyenne = "moyenne" // moyenne des 3 derniers mois complets
)

type EntreeTresorerie struct {
	Aujourdhui  time.Time
	Profil      Profil
	Saisies     []Transaction // au moins les 12 mois passés
	Echeances   []Echeance    // agendas couvrant la période et la prévision
	Recurrentes []Recurrente
	MRR         *int64 // en euros ; nil sans Stripe
	Solde       *int64 // solde du compte noté le jour SoldeAu
	SoldeAu     time.Time
	MoisPasses  int // mois affichés avant le mois en cours
	MoisFuturs  int // mois prévus après le mois en cours
}

type Tresorerie struct {
	Mois            []MoisTresorerie `json:"mois"`
	Methode         string           `json:"methode"`
	RecettesPrevues int64            `json:"recettes_prevues"` // par mois
	MoisMoyenne     int              `json:"mois_moyenne"`     // mois complets dans la moyenne (0 à 3)
	Moyenne         int64            `json:"moyenne"`          // moyenne des recettes de ces mois
	TauxCharges     float64          `json:"taux_charges"`     // cotisations, CFP et VL par euro encaissé, mois suivant
	FluxPrevu       int64            `json:"flux_prevu"`       // somme des mois prévus
	SoldePrevu      *int64           `json:"solde_prevu,omitempty"`
}

func cleMois(t time.Time) string { return t.Format("2006-01") }

// TauxCharges renvoie ce que coûte en cotisations, CFP et versement libératoire un euro encaissé
// le jour donné, ACRE comprise.
func TauxCharges(p Profil, set *bareme.Set, jour time.Time) (float64, error) {
	const test = 1_000_000
	r, err := Calculer(jour.Year(), p, []Transaction{{Type: Recette, Date: jour, Centimes: test}}, set)
	if err != nil {
		return 0, err
	}
	t := r.Total
	return float64(t.Cotisations+t.CFP+t.ImpotVL) / test, nil
}

// CalculerTresorerie résume les mois passés à partir des saisies et prévoit les mois suivants :
// recettes au MRR Stripe s'il est connu, sinon à la moyenne des 3 derniers mois complets ;
// dépenses récurrentes à leurs dates ; cotisations à chaque date limite URSSAF, estimées sur les
// recettes déjà saisies de la période et sur les recettes prévues.
func CalculerTresorerie(e EntreeTresorerie, set *bareme.Set) (*Tresorerie, error) {
	auj := e.Aujourdhui
	courant := time.Date(auj.Year(), auj.Month(), 1, 0, 0, 0, 0, time.UTC)
	premier := courant.AddDate(0, -e.MoisPasses, 0)
	dernierMois := courant.AddDate(0, e.MoisFuturs, 0)
	finHorizon := dernierMois.AddDate(0, 1, -1)

	idx := map[string]*MoisTresorerie{}
	var mois []*MoisTresorerie
	for m := premier; !m.After(dernierMois); m = m.AddDate(0, 1, 0) {
		mt := &MoisTresorerie{
			Mois: cleMois(m), Libelle: fmt.Sprintf("%s %d", moisFR[m.Month()-1], m.Year()),
			EnCours: m.Equal(courant), Prevision: m.After(courant),
		}
		idx[mt.Mois] = mt
		mois = append(mois, mt)
	}

	// Ce qui, dans le mois du solde noté, arrive après le jour du solde : le solde ne le contient pas encore.
	moisSolde := ""
	if e.Solde != nil {
		moisSolde = cleMois(e.SoldeAu)
	}
	var apresSolde int64
	noter := func(d time.Time, signe int64) {
		if cleMois(d) == moisSolde && d.After(e.SoldeAu) {
			apresSolde += signe
		}
	}

	// Réel : saisies jusqu'à aujourd'hui.
	recettesParMois := map[string]int64{}
	for _, t := range e.Saisies {
		if t.Date.After(auj) {
			continue
		}
		k := cleMois(t.Date)
		if t.Type == Recette {
			recettesParMois[k] += t.Centimes
		}
		mt := idx[k]
		if mt == nil || mt.Prevision {
			continue
		}
		switch {
		case t.Type == Recette:
			mt.Encaisse += t.Centimes
			noter(t.Date, t.Centimes)
		case t.Echeance != "":
			// Paiement URSSAF réel : il remplace l'estimation de sa déclaration.
			mt.Cotisations += t.Centimes
			noter(t.Date, -t.Centimes)
		default:
			mt.Depenses += t.Centimes
			noter(t.Date, -t.Centimes)
		}
	}

	// Recettes prévues par mois.
	res := &Tresorerie{}
	// Moyenne des saisies, toujours calculée pour pouvoir la comparer au MRR. Seuls comptent les
	// mois entiers d'activité : un mois commencé en cours de route ou d'avant le début la ferait baisser.
	debut := e.Profil.DebutActivite
	var somme int64
	for i := 1; i <= 3; i++ {
		m := courant.AddDate(0, -i, 0)
		if !debut.IsZero() && m.Before(time.Date(debut.Year(), debut.Month(), debut.Day(), 0, 0, 0, 0, time.UTC)) {
			break
		}
		somme += recettesParMois[cleMois(m)]
		res.MoisMoyenne++
	}
	if res.MoisMoyenne > 0 {
		res.Moyenne = int64(math.Round(float64(somme) / float64(res.MoisMoyenne)))
	}
	res.Methode, res.RecettesPrevues = PrevisionMoyenne, res.Moyenne
	if e.MRR != nil {
		res.Methode, res.RecettesPrevues = PrevisionMRR, *e.MRR
	}
	prevues := map[string]int64{}
	for _, mt := range mois {
		if mt.Prevision {
			mt.Encaisse = res.RecettesPrevues
			prevues[mt.Mois] = res.RecettesPrevues
		}
	}

	// Dépenses récurrentes à venir, y compris d'ici la fin du mois en cours : les dates passées
	// sont déjà des saisies.
	demain := auj.AddDate(0, 0, 1)
	for _, r := range e.Recurrentes {
		for _, d := range r.Occurrences(demain, finHorizon) {
			if mt := idx[cleMois(d)]; mt != nil {
				mt.Depenses += r.Centimes
				noter(d, -r.Centimes)
			}
		}
	}

	// Cotisations au mois de leur date limite.
	taux := map[string]float64{}
	tauxDe := func(m time.Time) (float64, error) {
		k := cleMois(m)
		if v, ok := taux[k]; ok {
			return v, nil
		}
		v, err := TauxCharges(e.Profil, set, m)
		taux[k] = v
		return v, err
	}
	vues := map[string]bool{}
	for _, ec := range e.Echeances {
		if ec.Type != "urssaf" || ec.Date == "" || vues[ec.Code] {
			continue
		}
		vues[ec.Code] = true
		d, err := time.Parse("2006-01-02", ec.Date)
		if err != nil {
			return nil, err
		}
		mt := idx[cleMois(d)]
		if mt == nil {
			continue
		}
		montant := ec.APayer
		if debut, err := time.Parse("2006-01-02", ec.PeriodeDebut); err == nil {
			fin, _ := time.Parse("2006-01-02", ec.PeriodeFin)
			for m := time.Date(debut.Year(), debut.Month(), 1, 0, 0, 0, 0, time.UTC); !m.After(fin); m = m.AddDate(0, 1, 0) {
				if p := prevues[cleMois(m)]; p > 0 {
					t, err := tauxDe(m)
					if err != nil {
						return nil, err
					}
					montant += int64(math.Round(float64(p) * t))
				}
			}
		}
		mt.Cotisations += montant
		noter(d, -montant)
	}

	// Flux et solde.
	if t, err := tauxDe(courant.AddDate(0, 1, 0)); err == nil {
		res.TauxCharges = t
	}
	var solde *int64
	if e.Solde != nil {
		v := *e.Solde
		solde = &v
	}
	for _, mt := range mois {
		mt.Flux = mt.Encaisse - mt.Depenses - mt.Cotisations
		if mt.Prevision {
			res.FluxPrevu += mt.Flux
		}
		if solde == nil || mt.Mois < moisSolde {
			continue
		}
		// Fin du mois du solde : le solde noté plus ce qui arrive après ; ensuite, chaque mois ajoute son flux.
		if mt.Mois == moisSolde {
			*solde += apresSolde
		} else {
			*solde += mt.Flux
		}
		v := *solde
		mt.SoldeFin = &v
	}
	if solde != nil {
		v := *solde
		res.SoldePrevu = &v
		// Avant le mois du solde, on remonte le temps : la fin d'un mois est la fin du suivant
		// moins le flux du suivant. La vue cumulée a ainsi un solde pour chaque mois.
		for i := len(mois) - 2; i >= 0; i-- {
			if mois[i].SoldeFin == nil && mois[i+1].SoldeFin != nil {
				v := *mois[i+1].SoldeFin - mois[i+1].Flux
				mois[i].SoldeFin = &v
			}
		}
	}
	res.Mois = make([]MoisTresorerie, len(mois))
	for i, mt := range mois {
		res.Mois[i] = *mt
	}
	return res, nil
}
