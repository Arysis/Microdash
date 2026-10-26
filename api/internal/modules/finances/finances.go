// Package finances est le module Finances du copilote : il met dans « Ce soir » les échéances
// URSSAF et fiscales proches, les plafonds qui approchent et les opérations Qonto à valider.
package finances

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/arysis/microdash/api/internal/bareme"
	"github.com/arysis/microdash/api/internal/calc"
	"github.com/arysis/microdash/api/internal/modules"
	"github.com/arysis/microdash/api/internal/store"
)

// Une échéance apparaît dans « Ce soir » 14 jours avant sa date, et reste tant qu'elle
// n'est pas faite (jusqu'à 60 jours de retard).
const (
	joursAvant  = 14
	joursApres  = 60
	formatDate  = "2006-01-02"
	codeSource  = "finances"
	refQonto    = "qonto-a-valider"
	prefixePlaf = "plafond-"
	lienAgenda  = "/echeances"
)

// Donnees est ce que le module lit et écrit en base.
type Donnees interface {
	Profil(ctx context.Context, userID int64) (*store.Profil, error)
	Transactions(ctx context.Context, userID int64, du, au time.Time) ([]store.Transaction, error)
	EcheancesFaites(ctx context.Context, userID int64) (map[string]time.Time, error)
	MarquerEcheance(ctx context.Context, userID int64, code string, faite bool) error
	CompterAValider(ctx context.Context, userID int64) (int, error)
}

type Module struct {
	Donnees Donnees
	Baremes *bareme.Set
}

func (Module) Code() string { return codeSource }
func (Module) Nom() string  { return "Finances" }

func (m Module) ATraiter(ctx context.Context, uid int64, aujourdhui time.Time) ([]modules.Element, error) {
	p, err := m.Donnees.Profil(ctx, uid)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil // sans profil, aucun calcul possible
	}
	if err != nil {
		return nil, err
	}
	annee := aujourdhui.Year()
	echs, txs, err := m.echeances(ctx, uid, p, annee-1, annee)
	if err != nil {
		return nil, err
	}
	faites, err := m.Donnees.EcheancesFaites(ctx, uid)
	if err != nil {
		return nil, err
	}
	var res []modules.Element
	for _, e := range echs {
		if el, ok := echeance(e, faites, aujourdhui); ok {
			res = append(res, el)
		}
	}

	if !p.DebutActivite.After(aujourdhui) {
		if r, err := calc.Calculer(annee, p.VersCalc(), store.VersCalc(txs), m.Baremes); err == nil {
			res = append(res, plafonds(r)...)
		}
	}

	n, err := m.Donnees.CompterAValider(ctx, uid)
	if err != nil {
		return nil, err
	}
	if n > 0 {
		titre := "1 opération Qonto à valider"
		if n > 1 {
			titre = fmt.Sprintf("%d opérations Qonto à valider", n)
		}
		res = append(res, modules.Element{
			Source: codeSource, Ref: refQonto, Titre: titre,
			Detail: "Crée la saisie ou ignore l'opération.", Lien: "/saisies",
		})
	}
	return res, nil
}

// echeances renvoie les échéances des années de premiere à derniere (une échéance de fin
// d'année tombe l'année d'après : elle n'est comptée qu'une fois), et les saisies lues.
func (m Module) echeances(ctx context.Context, uid int64, p *store.Profil, premiere, derniere int) ([]calc.Echeance, []store.Transaction, error) {
	// Les saisies de l'année d'avant servent aux échéances de janvier et février (fin d'année
	// précédente), celles de l'année d'après aux paiements URSSAF qu'elles contiennent.
	txs, err := m.Donnees.Transactions(ctx, uid, time.Date(premiere-1, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(derniere+1, 12, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		return nil, nil, err
	}
	var res []calc.Echeance
	vues := map[string]bool{}
	for a := premiere; a <= derniere; a++ {
		agenda, err := calc.Agenda(a, p.VersCalc(), p.Periodicite, store.VersCalc(txs), m.Baremes)
		if err != nil {
			continue // pas de barème pour cette année : rien à montrer
		}
		for _, e := range agenda {
			if !vues[e.Code] {
				vues[e.Code] = true
				res = append(res, e)
			}
		}
	}
	return res, txs, nil
}

// Evenements place les échéances URSSAF et fiscales dans l'agenda.
func (m Module) Evenements(ctx context.Context, uid int64, du, au time.Time) ([]modules.Evenement, error) {
	p, err := m.Donnees.Profil(ctx, uid)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	echs, _, err := m.echeances(ctx, uid, p, du.Year()-1, au.Year())
	if err != nil {
		return nil, err
	}
	faites, err := m.Donnees.EcheancesFaites(ctx, uid)
	if err != nil {
		return nil, err
	}
	debut, fin := du.Format(formatDate), au.Format(formatDate)
	var res []modules.Evenement
	for _, e := range echs {
		if e.Date == "" || e.Date < debut || e.Date > fin {
			continue
		}
		_, faite := faites[e.Code]
		res = append(res, modules.Evenement{
			Source: codeSource, Ref: e.Code, Titre: e.Libelle, Date: e.Date,
			Type: "echeance", Fait: faite || e.Paye > 0, Lien: lienAgenda,
		})
	}
	return res, nil
}

// echeance transforme une échéance de l'agenda en élément, si elle est proche et pas encore faite.
func echeance(e calc.Echeance, faites map[string]time.Time, aujourdhui time.Time) (modules.Element, bool) {
	if e.Date == "" || e.Paye > 0 {
		return modules.Element{}, false
	}
	if _, ok := faites[e.Code]; ok {
		return modules.Element{}, false
	}
	date, err := time.Parse(formatDate, e.Date)
	if err != nil {
		return modules.Element{}, false
	}
	jours := int(date.Sub(aujourdhui).Hours() / 24)
	if jours > joursAvant || jours < -joursApres {
		return modules.Element{}, false
	}
	el := modules.Element{
		Source: codeSource, Ref: e.Code, Titre: e.Libelle, Date: e.Date, Limite: true, Lien: lienAgenda, Finir: true,
	}
	switch {
	case jours <= 2:
		el.Importance = 2
	case jours <= 7:
		el.Importance = 1
	}
	if e.Type == "urssaf" {
		el.Detail = fmt.Sprintf("Chiffre d'affaires à déclarer : %s. Cotisations estimées : %s.", calc.Euros(e.CA), calc.Euros(e.APayer))
	} else {
		el.Detail = e.Note
	}
	return el, true
}

// plafonds renvoie un élément par plafond à 80 % ou dépassé. Le niveau fait partie de la
// référence : un plafond marqué « Fait » à 80 % revient s'il est ensuite dépassé.
func plafonds(r *calc.Resultat) []modules.Element {
	var res []modules.Element
	for _, p := range r.Plafonds {
		el := modules.Element{
			Source: codeSource, Ref: fmt.Sprintf("%s%d-%s-%s", prefixePlaf, r.Annee, p.Code, p.Niveau),
			Detail: fmt.Sprintf("%s sur un seuil de %s (%d %%), selon tes saisies.", calc.Euros(p.CA), calc.Euros(p.Plafond), int(math.Floor(p.Ratio*100))),
			Lien:   "/tableau-de-bord", Finir: true,
		}
		switch p.Niveau {
		case "depasse":
			el.Titre, el.Importance = "Seuil dépassé : "+p.Libelle, 2
		case "attention":
			el.Titre, el.Importance = "Tu approches du seuil : "+p.Libelle, 1
		default:
			continue
		}
		res = append(res, el)
	}
	return res
}

// Finir coche l'échéance dans l'agenda. Pour un plafond, « Fait » veut seulement dire « vu ».
func (m Module) Finir(ctx context.Context, uid int64, ref string) error {
	if ref == refQonto || strings.HasPrefix(ref, prefixePlaf) {
		return nil
	}
	return m.Donnees.MarquerEcheance(ctx, uid, ref, true)
}
