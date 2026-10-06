package calc

import (
	"fmt"
	"time"

	"github.com/arysis/microdash/api/internal/bareme"
)

const (
	Mensuelle     = "mensuelle"
	Trimestrielle = "trimestrielle"
)

// Echeance est une date limite administrative de l'agenda. Les montants sont en centimes.
type Echeance struct {
	Code         string `json:"code"` // identifiant stable, ex. urssaf-2026-t3, revenus-2025, cfe-2026
	Type         string `json:"type"` // urssaf, revenus, cfe
	Libelle      string `json:"libelle"`
	Date         string `json:"date,omitempty"`        // date limite après report ; vide si inconnue
	DateLegale   string `json:"date_legale,omitempty"` // date avant report, si un week-end ou un férié l'a décalée
	PeriodeDebut string `json:"periode_debut,omitempty"`
	PeriodeFin   string `json:"periode_fin,omitempty"`
	CA           int64  `json:"ca"`
	APayer       int64  `json:"a_payer"` // cotisations, CFP et versement libératoire estimés
	Note         string `json:"note,omitempty"`
	BaremeExact  bool   `json:"bareme_exact"` // faux si un montant utilise le barème d'une autre année
}

// periode est une période de déclaration URSSAF (un mois ou un trimestre).
type periode struct{ debut, fin time.Time }

func date(a int, m time.Month, j int) time.Time { return time.Date(a, m, j, 0, 0, 0, 0, time.UTC) }

// paques renvoie le dimanche de Pâques (algorithme de Meeus pour le calendrier grégorien).
func paques(a int) time.Time {
	b, c := a/100, a%100
	d, e := b/4, b%4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*(a%19) + b - d - g + 15) % 30
	i, k := c/4, c%4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a%19 + 11*h + 22*l) / 451
	mois := (h + l - 7*m + 114) / 31
	jour := (h+l-7*m+114)%31 + 1
	return date(a, time.Month(mois), jour)
}

// Ferie indique si le jour est un jour férié national en France métropolitaine.
func Ferie(t time.Time) bool {
	a := t.Year()
	p := paques(a)
	for _, f := range []time.Time{
		date(a, 1, 1), p.AddDate(0, 0, 1), date(a, 5, 1), date(a, 5, 8), p.AddDate(0, 0, 39),
		p.AddDate(0, 0, 50), date(a, 7, 14), date(a, 8, 15), date(a, 11, 1), date(a, 11, 11), date(a, 12, 25),
	} {
		if t.Equal(f) {
			return true
		}
	}
	return false
}

// JourOuvre reporte une date limite qui tombe un samedi, un dimanche ou un jour férié
// au premier jour ouvré qui suit (règle de l'URSSAF).
func JourOuvre(t time.Time) time.Time {
	for t.Weekday() == time.Saturday || t.Weekday() == time.Sunday || Ferie(t) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

// limite renvoie la date limite d'une période : le dernier jour du mois qui suit sa fin.
func limite(p periode) time.Time {
	return date(p.fin.Year(), p.fin.Month(), 1).AddDate(0, 2, -1)
}

func periodeDe(t time.Time, periodicite string) periode {
	if periodicite == Mensuelle {
		d := date(t.Year(), t.Month(), 1)
		return periode{d, d.AddDate(0, 1, -1)}
	}
	d := date(t.Year(), time.Month((int(t.Month())-1)/3*3+1), 1)
	return periode{d, d.AddDate(0, 3, -1)}
}

func suivante(p periode, periodicite string) periode {
	return periodeDe(p.fin.AddDate(0, 0, 1), periodicite)
}

var trimestresFR = []string{"1er", "2e", "3e", "4e"}

func libelleURSSAF(p periode, periodicite string, premiere bool) string {
	if premiere {
		return "Première déclaration URSSAF"
	}
	if periodicite == Mensuelle {
		return fmt.Sprintf("Déclaration URSSAF de %s %d", moisFR[p.fin.Month()-1], p.fin.Year())
	}
	return fmt.Sprintf("Déclaration URSSAF du %s trimestre %d", trimestresFR[(int(p.fin.Month())-1)/3], p.fin.Year())
}

func codeURSSAF(p periode, periodicite string) string {
	if periodicite == Mensuelle {
		return fmt.Sprintf("urssaf-%d-%02d", p.fin.Year(), p.fin.Month())
	}
	return fmt.Sprintf("urssaf-%d-t%d", p.fin.Year(), (int(p.fin.Month())-1)/3+1)
}

// declarationsURSSAF liste les déclarations dont la date limite légale tombe dans l'année.
// La première déclaration regroupe la période du début d'activité et les suivantes, pour
// respecter le délai de 90 jours : jusqu'à la fin du 3e mois qui suit le mois de début
// (mensuel) ou jusqu'à la fin du trimestre qui suit (trimestriel).
func declarationsURSSAF(annee int, debut time.Time, periodicite string) []periode {
	var res []periode
	p := periodeDe(debut, periodicite)
	regroupees := 1
	if periodicite == Mensuelle {
		regroupees = 3
	}
	fin := p
	for i := 0; i < regroupees; i++ {
		fin = suivante(fin, periodicite)
	}
	courante := periode{debut, fin.fin}
	for {
		l := limite(courante)
		if l.Year() > annee {
			return res
		}
		if l.Year() == annee {
			res = append(res, courante)
		}
		courante = suivante(periode{courante.fin, courante.fin}, periodicite)
	}
}

// montants additionne le chiffre d'affaires et ce qu'il y a à payer sur une période,
// à partir des totaux mensuels de chaque année concernée.
func montants(p periode, parAnnee map[int]*Resultat) (ca, aPayer int64, exact bool) {
	exact = true
	for m := date(p.debut.Year(), p.debut.Month(), 1); !m.After(p.fin); m = m.AddDate(0, 1, 0) {
		r := parAnnee[m.Year()]
		if r == nil {
			continue
		}
		exact = exact && r.BaremeExact
		t := r.Mois[m.Month()-1].Totaux
		ca += t.CA
		aPayer += t.Cotisations + t.CFP + t.ImpotVL
	}
	return ca, aPayer, exact
}

// Agenda liste les échéances dont la date limite tombe dans l'année, dans l'ordre des dates.
// Une échéance sans date connue (déclaration de revenus) est placée à sa période habituelle.
func Agenda(annee int, p Profil, periodicite string, txs []Transaction, set *bareme.Set) ([]Echeance, error) {
	if periodicite != Mensuelle && periodicite != Trimestrielle {
		return nil, fmt.Errorf("périodicité inconnue : %q", periodicite)
	}
	debut := p.DebutActivite
	if debut.IsZero() {
		return nil, fmt.Errorf("date de début d'activité manquante")
	}
	parAnnee := map[int]*Resultat{}
	for a := annee - 2; a <= annee; a++ {
		if a < debut.Year() {
			continue
		}
		r, err := Calculer(a, p, txs, set)
		if err != nil {
			return nil, err
		}
		parAnnee[a] = r
	}

	var res []Echeance
	for _, per := range declarationsURSSAF(annee, debut, periodicite) {
		legale := limite(per)
		reportee := JourOuvre(legale)
		ca, aPayer, exact := montants(per, parAnnee)
		e := Echeance{
			Code: codeURSSAF(per, periodicite), Type: "urssaf",
			Libelle:      libelleURSSAF(per, periodicite, per.debut.Equal(debut)),
			Date:         reportee.Format("2006-01-02"),
			PeriodeDebut: per.debut.Format("2006-01-02"), PeriodeFin: per.fin.Format("2006-01-02"),
			CA: ca, APayer: aPayer, BaremeExact: exact,
		}
		if !reportee.Equal(legale) {
			e.DateLegale = legale.Format("2006-01-02")
		}
		if ca == 0 {
			e.Note = "Aucune recette sur la période : la déclaration est due quand même, à 0 €."
		}
		res = append(res, e)
	}

	b, exact := set.Pour(annee)
	// Déclaration de revenus de l'année précédente : la date dépend du département.
	if debut.Year() < annee {
		e := Echeance{
			Code: fmt.Sprintf("revenus-%d", annee-1), Type: "revenus",
			Libelle: fmt.Sprintf("Déclaration de revenus %d", annee-1), BaremeExact: true,
		}
		if r := parAnnee[annee-1]; r != nil {
			e.CA = r.Total.CA
		}
		// La date publiée pour une autre année ne vaut pas pour celle-ci.
		if exact && b.Echeances.DeclarationRevenus != nil {
			e.Date = b.Echeances.DeclarationRevenus.Format("2006-01-02")
			e.Note = "Date de la zone la plus tôt. Vérifie celle de ton département sur impots.gouv.fr."
		} else {
			e.Note = "Fin mai ou début juin, selon ton département."
		}
		res = append(res, e)
	}
	// CFE : pas due l'année de création de l'entreprise.
	if debut.Year() < annee && b.Echeances.CFE.Mois > 0 {
		res = append(res, Echeance{
			Code: fmt.Sprintf("cfe-%d", annee), Type: "cfe",
			Libelle:     fmt.Sprintf("CFE %d", annee),
			Date:        date(annee, time.Month(b.Echeances.CFE.Mois), b.Echeances.CFE.Jour).Format("2006-01-02"),
			Note:        "À confirmer pour ton cas : le montant et les exonérations dépendent de ta commune.",
			BaremeExact: exact,
		})
	}
	trier(res, annee)
	return res, nil
}

// trier range les échéances par date ; une date inconnue se place au 1er juin, sa période habituelle.
func trier(es []Echeance, annee int) {
	cle := func(e Echeance) string {
		if e.Date != "" {
			return e.Date
		}
		return fmt.Sprintf("%d-06-01", annee)
	}
	for i := 1; i < len(es); i++ {
		for j := i; j > 0 && cle(es[j]) < cle(es[j-1]); j-- {
			es[j], es[j-1] = es[j-1], es[j]
		}
	}
}
