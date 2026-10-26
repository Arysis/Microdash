// Package modules définit ce qu'un module du copilote apporte à l'écran « Ce soir » :
// une liste d'éléments à traiter, au même format pour toutes les sources.
package modules

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// Element est une chose à traiter, venue d'un module (une échéance, un mail, une tâche…).
type Element struct {
	Source     string `json:"source"` // code du module
	Ref        string `json:"ref"`    // identifiant stable dans le module
	Titre      string `json:"titre"`
	Detail     string `json:"detail,omitempty"`
	Date       string `json:"date,omitempty"`   // AAAA-MM-JJ : date limite ou de réception
	Limite     bool   `json:"limite,omitempty"` // vrai si Date est une date limite
	Importance int    `json:"importance"`       // 0 normal, 1 important, 2 urgent
	Lien       string `json:"lien,omitempty"`   // page de l'app (/agenda) ou adresse externe (https://…)
	Finir      bool   `json:"finir"`            // vrai si « Fait » a un sens pour cet élément
}

// Cle identifie un élément parmi toutes les sources.
func (e Element) Cle() string { return e.Source + "/" + e.Ref }

// Module est une partie du copilote qu'on peut activer ou désactiver.
type Module interface {
	Code() string
	Nom() string
}

// Source est un module qui met des éléments dans « Ce soir ».
type Source interface {
	Module
	ATraiter(ctx context.Context, userID int64, aujourdhui time.Time) ([]Element, error)
}

// Evenement est une chose datée, montrée dans l'agenda et dans le flux iCal.
type Evenement struct {
	Source string `json:"source"`
	Ref    string `json:"ref"`
	Titre  string `json:"titre"`
	Date   string `json:"date"`            // AAAA-MM-JJ
	Heure  string `json:"heure,omitempty"` // HH:MM, heure de Paris ; vide : toute la journée
	Duree  int    `json:"duree,omitempty"` // minutes, pour un événement à heure fixe
	Type   string `json:"type"`            // evenement, tache, echeance
	Fait   bool   `json:"fait,omitempty"`
	Lien   string `json:"lien,omitempty"`
}

// Calendrier est un module qui place des choses dans l'agenda.
type Calendrier interface {
	Module
	Evenements(ctx context.Context, userID int64, du, au time.Time) ([]Evenement, error)
}

// Finisseur est un module qui agit quand la personne marque un de ses éléments « Fait »
// (par exemple, cocher l'échéance dans l'agenda).
type Finisseur interface {
	Finir(ctx context.Context, userID int64, ref string) error
}

// Registre liste les modules du copilote, dans l'ordre d'affichage.
type Registre struct{ liste []Module }

func NewRegistre(ms ...Module) *Registre { return &Registre{liste: ms} }

func (r *Registre) Modules() []Module { return r.liste }

func (r *Registre) Module(code string) (Module, bool) {
	for _, m := range r.liste {
		if m.Code() == code {
			return m, true
		}
	}
	return nil, false
}

// Etat est ce que la personne a décidé pour un élément : fait, ou reporté jusqu'à une date.
type Etat struct {
	Statut string    // fait, reporte
	Jusqua time.Time // pour un report : l'élément revient ce jour-là
}

// Filtrer retire les éléments faits et ceux reportés à plus tard qu'aujourd'hui.
func Filtrer(els []Element, etats map[string]Etat, aujourdhui time.Time) []Element {
	res := make([]Element, 0, len(els))
	for _, e := range els {
		if et, ok := etats[e.Cle()]; ok {
			if et.Statut == "fait" || et.Jusqua.After(aujourdhui) {
				continue
			}
		}
		res = append(res, e)
	}
	return res
}

// Trier range les éléments du plus pressant au moins pressant : importance, puis date
// (les éléments sans date à la fin), puis titre.
func Trier(els []Element) {
	sort.SliceStable(els, func(i, j int) bool {
		a, b := els[i], els[j]
		if a.Importance != b.Importance {
			return a.Importance > b.Importance
		}
		if (a.Date == "") != (b.Date == "") {
			return a.Date != ""
		}
		if a.Date != b.Date {
			return a.Date < b.Date
		}
		return a.Titre < b.Titre
	})
}

// Reporter renvoie le jour où revient un élément remis à plus tard : demain, samedi prochain
// ou lundi prochain.
func Reporter(quand string, aujourdhui time.Time) (time.Time, error) {
	switch quand {
	case "demain":
		return aujourdhui.AddDate(0, 0, 1), nil
	case "week-end":
		n := (int(time.Saturday) - int(aujourdhui.Weekday()) + 7) % 7
		if n == 0 { // déjà samedi : le week-end suivant (le dimanche mène au samedi d'après)
			n = 7
		}
		return aujourdhui.AddDate(0, 0, n), nil
	case "semaine":
		n := (int(time.Monday) - int(aujourdhui.Weekday()) + 7) % 7
		if n == 0 {
			n = 7
		}
		return aujourdhui.AddDate(0, 0, n), nil
	}
	return time.Time{}, fmt.Errorf("report inconnu : %q (demain, week-end ou semaine)", quand)
}
