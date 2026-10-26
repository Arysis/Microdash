// Package ical écrit un flux iCalendar (RFC 5545) lisible par Home Assistant, Google Agenda
// ou le calendrier du téléphone.
package ical

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Evenement est une entrée du flux. Sans heure, il occupe toute la journée.
type Evenement struct {
	UID    string
	Titre  string
	Notes  string
	Date   time.Time // jour, à minuit UTC
	Heure  string    // HH:MM, heure de Paris ; vide : toute la journée
	Duree  int       // minutes ; 60 par défaut pour un événement à heure fixe
	Statut string    // vide, ou CANCELLED / CONFIRMED…
}

var paris = func() *time.Location {
	l, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		panic(err)
	}
	return l
}()

// Ecrire renvoie le calendrier complet, lignes terminées par CRLF.
func Ecrire(nom string, evs []Evenement, maintenant time.Time) string {
	var b strings.Builder
	ligne := func(s string) { b.WriteString(plier(s)) }
	ligne("BEGIN:VCALENDAR")
	ligne("VERSION:2.0")
	ligne("PRODID:-//Microdash//Copilote//FR")
	ligne("CALSCALE:GREGORIAN")
	ligne("METHOD:PUBLISH")
	ligne("X-WR-CALNAME:" + echapper(nom))
	ligne("X-WR-TIMEZONE:Europe/Paris")
	stamp := maintenant.UTC().Format("20060102T150405Z")
	for _, e := range evs {
		ligne("BEGIN:VEVENT")
		ligne("UID:" + echapper(e.UID))
		ligne("DTSTAMP:" + stamp)
		if debut, ok := heureParis(e.Date, e.Heure); ok {
			d := e.Duree
			if d <= 0 {
				d = 60
			}
			ligne("DTSTART:" + debut.UTC().Format("20060102T150405Z"))
			ligne("DTEND:" + debut.Add(time.Duration(d)*time.Minute).UTC().Format("20060102T150405Z"))
		} else {
			ligne("DTSTART;VALUE=DATE:" + e.Date.Format("20060102"))
			ligne("DTEND;VALUE=DATE:" + e.Date.AddDate(0, 0, 1).Format("20060102"))
			ligne("TRANSP:TRANSPARENT")
		}
		ligne("SUMMARY:" + echapper(e.Titre))
		if e.Notes != "" {
			ligne("DESCRIPTION:" + echapper(e.Notes))
		}
		if e.Statut != "" {
			ligne("STATUS:" + e.Statut)
		}
		ligne("END:VEVENT")
	}
	ligne("END:VCALENDAR")
	return b.String()
}

// heureParis place l'heure de Paris le jour donné (changements d'heure compris).
func heureParis(jour time.Time, heure string) (time.Time, bool) {
	if heure == "" {
		return time.Time{}, false
	}
	var h, m int
	if _, err := fmt.Sscanf(heure, "%d:%d", &h, &m); err != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return time.Time{}, false
	}
	return time.Date(jour.Year(), jour.Month(), jour.Day(), h, m, 0, 0, paris), true
}

// echapper protège les caractères spéciaux d'un texte iCalendar.
func echapper(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\n", `\n`, "\r", "").Replace(s)
}

// plier coupe une ligne à 75 octets au plus, sans couper un caractère UTF-8 : les lignes
// suivantes commencent par une espace.
func plier(s string) string {
	var b strings.Builder
	max := 75
	for len(s) > max {
		n := max
		for n > 0 && !utf8.RuneStart(s[n]) {
			n--
		}
		b.WriteString(s[:n])
		b.WriteString("\r\n ")
		s = s[n:]
		max = 74 // l'espace de continuation compte
	}
	b.WriteString(s)
	b.WriteString("\r\n")
	return b.String()
}
