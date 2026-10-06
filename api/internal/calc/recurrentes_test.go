package calc

import (
	"strings"
	"testing"
	"time"
)

func dates(ts []time.Time) string {
	var s []string
	for _, t := range ts {
		s = append(s, t.Format("2006-01-02"))
	}
	return strings.Join(s, ",")
}

func TestOccurrences(t *testing.T) {
	fin := jour("2026-05-15")
	cas := []struct {
		r        Recurrente
		du, au   string
		attendus string
	}{
		{Recurrente{Frequence: Mensuelle, Debut: jour("2026-01-31")}, "2026-01-01", "2026-05-31", "2026-01-31,2026-02-28,2026-03-31,2026-04-30,2026-05-31"},
		{Recurrente{Frequence: Mensuelle, Debut: jour("2025-11-10")}, "2026-01-01", "2026-03-09", "2026-01-10,2026-02-10"},
		{Recurrente{Frequence: Mensuelle, Debut: jour("2026-01-10"), Fin: &fin}, "2026-01-01", "2026-12-31", "2026-01-10,2026-02-10,2026-03-10,2026-04-10,2026-05-10"},
		{Recurrente{Frequence: Annuelle, Debut: jour("2024-02-29")}, "2024-01-01", "2028-12-31", "2024-02-29,2025-02-28,2026-02-28,2027-02-28,2028-02-29"},
		{Recurrente{Frequence: Mensuelle, Debut: jour("2026-12-01")}, "2026-01-01", "2026-11-30", ""},
	}
	for _, c := range cas {
		if got := dates(c.r.Occurrences(jour(c.du), jour(c.au))); got != c.attendus {
			t.Errorf("%+v [%s, %s] = %s, attendu %s", c.r, c.du, c.au, got, c.attendus)
		}
	}
}
