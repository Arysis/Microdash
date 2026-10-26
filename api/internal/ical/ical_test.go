package ical

import (
	"strings"
	"testing"
	"time"
)

func TestEcrire(t *testing.T) {
	jour := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	ete := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	cal := Ecrire("Microdash", []Evenement{
		{UID: "finances-urssaf@microdash", Titre: "Déclaration URSSAF, 3e trimestre; à faire", Date: jour},
		{UID: "agenda-1@microdash", Titre: "Appel comptable", Date: jour, Heure: "18:30", Duree: 45},
		{UID: "agenda-2@microdash", Titre: "Été", Date: ete, Heure: "09:00", Notes: "ligne 1\nligne 2"},
	}, time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC))

	for _, attendu := range []string{
		"BEGIN:VCALENDAR\r\n",
		"DTSTART;VALUE=DATE:20261102\r\nDTEND;VALUE=DATE:20261103\r\n",
		`SUMMARY:Déclaration URSSAF\, 3e trimestre\; à faire`,
		"DTSTART:20261102T173000Z\r\nDTEND:20261102T181500Z\r\n", // heure d'hiver : UTC+1
		"DTSTART:20260701T070000Z\r\nDTEND:20260701T080000Z\r\n", // heure d'été : UTC+2, 1 h par défaut
		`DESCRIPTION:ligne 1\nligne 2`,
		"END:VCALENDAR\r\n",
	} {
		if !strings.Contains(cal, attendu) {
			t.Errorf("manque %q dans :\n%s", attendu, cal)
		}
	}
}

func TestPlier(t *testing.T) {
	long := "SUMMARY:" + strings.Repeat("é", 100)
	plie := plier(long)
	for _, l := range strings.Split(strings.TrimSuffix(plie, "\r\n"), "\r\n") {
		if len(l) > 75 {
			t.Errorf("ligne de %d octets", len(l))
		}
		if !strings.HasPrefix(l, "SUMMARY") && !strings.HasPrefix(l, " ") {
			t.Errorf("ligne de suite sans espace : %q", l)
		}
	}
	if strings.ReplaceAll(strings.TrimSuffix(plie, "\r\n"), "\r\n ", "") != long {
		t.Error("le texte déplié diffère de l'original")
	}
}
