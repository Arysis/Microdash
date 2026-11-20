// Package notifs choisit les notifications push à envoyer : le résumé de « Ta soirée » à l'heure
// choisie, et un rappel avant chaque rendez-vous à heure fixe.
package notifs

import (
	"fmt"
	"strings"
	"time"

	"github.com/arysis/microdash/api/internal/modules"
	"github.com/arysis/microdash/api/internal/store"
)

// Paris est le fuseau des heures de l'agenda.
var Paris = func() *time.Location {
	l, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		panic(err)
	}
	return l
}()

// Fenêtres de rattrapage : après un redémarrage de l'API, une notification encore utile part
// en retard ; au-delà, elle est abandonnée.
const (
	fenetreSoiree = 3 * time.Hour
	fenetreRDV    = 10 * time.Minute
	lignesMax     = 4
)

// Notification est un message à pousser vers tous les appareils d'un compte.
type Notification struct {
	Cle     string        `json:"-"` // unique par compte : jamais envoyée deux fois
	Titre   string        `json:"titre"`
	Corps   string        `json:"corps"`
	Lien    string        `json:"lien"` // page de l'app ouverte au clic
	Tag     string        `json:"tag"`  // une notification du même tag remplace la précédente
	TTL     time.Duration `json:"-"`
	Urgence string        `json:"-"`
}

// Choisir renvoie les notifications dues à l'instant donné, pour les événements d'aujourd'hui
// et de demain (un rendez-vous juste après minuit se rappelle la veille).
func Choisir(evs []modules.Evenement, p store.PreferencesNotifications, maintenant time.Time) []Notification {
	maintenant = maintenant.In(Paris)
	aujourdhui := maintenant.Format("2006-01-02")
	var res []Notification

	if p.Soiree {
		if h, ok := aHeure(aujourdhui, p.HeureSoiree); ok && !maintenant.Before(h) && maintenant.Before(h.Add(fenetreSoiree)) {
			if n, ok := soiree(evs, aujourdhui, h); ok {
				n.TTL = h.Add(fenetreSoiree).Sub(maintenant)
				res = append(res, n)
			}
		}
	}
	if p.RendezVous {
		avance := time.Duration(p.Avance) * time.Minute
		for _, e := range evs {
			if e.Type != "evenement" || e.Heure == "" || e.Fait {
				continue
			}
			debut, ok := aHeure(e.Date, e.Heure)
			if !ok || maintenant.Before(debut.Add(-avance)) || !maintenant.Before(debut.Add(fenetreRDV)) {
				continue
			}
			corps := "À " + e.Heure
			if reste := debut.Sub(maintenant).Round(time.Minute); reste > 0 {
				corps += " (dans " + duree(reste) + ")"
			} else {
				corps += ", c'est maintenant"
			}
			if note := premiereLigne(e.Notes); note != "" {
				corps += "\n" + note
			}
			res = append(res, Notification{
				Cle: "rdv:" + e.Source + "-" + e.Ref + ":" + e.Date + "T" + e.Heure, Titre: e.Titre, Corps: corps,
				Lien: "/agenda", Tag: "rdv-" + e.Source + "-" + e.Ref, TTL: debut.Add(30 * time.Minute).Sub(maintenant), Urgence: "high",
			})
		}
	}
	return res
}

// soiree résume ce qui reste pour la soirée : tâches du jour, échéances, rendez-vous de la
// journée entière et ceux à heure fixe à partir de l'heure du résumé.
func soiree(evs []modules.Evenement, jour string, h time.Time) (Notification, bool) {
	var lignes []string
	taches := 0
	for _, e := range evs {
		if e.Date != jour || e.Fait {
			continue
		}
		switch {
		case e.Type == "tache":
			taches++
			lignes = append(lignes, e.Titre)
		case e.Type == "echeance":
			lignes = append(lignes, "À payer ou déclarer : "+e.Titre)
		case e.Heure == "":
			lignes = append(lignes, e.Titre)
		default:
			if d, ok := aHeure(e.Date, e.Heure); ok && !d.Before(h) {
				lignes = append(lignes, e.Heure+" "+e.Titre)
			}
		}
	}
	if len(lignes) == 0 {
		return Notification{}, false
	}
	if len(lignes) > lignesMax {
		autres := len(lignes) - (lignesMax - 1)
		lignes = append(lignes[:lignesMax-1], fmt.Sprintf("et %d autres", autres))
	}
	titre := "Ta soirée"
	switch {
	case taches == 1:
		titre += " : 1 tâche"
	case taches > 1:
		titre += fmt.Sprintf(" : %d tâches", taches)
	}
	return Notification{
		Cle: "soiree:" + jour, Titre: titre, Corps: strings.Join(lignes, "\n"),
		Lien: "/ce-soir", Tag: "soiree", Urgence: "normal",
	}, true
}

// aHeure place une heure HH:MM de Paris le jour AAAA-MM-JJ (changements d'heure compris).
func aHeure(jour, heure string) (time.Time, bool) {
	t, err := time.ParseInLocation("2006-01-02 15:04", jour+" "+heure, Paris)
	return t, err == nil
}

func duree(d time.Duration) string {
	m := int(d.Minutes())
	switch {
	case m < 60:
		return fmt.Sprintf("%d min", m)
	case m%60 == 0:
		return fmt.Sprintf("%d h", m/60)
	}
	return fmt.Sprintf("%d h %02d", m/60, m%60)
}

func premiereLigne(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	if r := []rune(s); len(r) > 120 {
		s = string(r[:119]) + "…"
	}
	return s
}
