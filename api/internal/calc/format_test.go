package calc

import "testing"

func TestEuros(t *testing.T) {
	cas := map[int64]string{0: "0,00 €", 5: "0,05 €", 109180: "1 091,80 €", 123456789: "1 234 567,89 €", -1500: "-15,00 €"}
	for c, attendu := range cas {
		if got := Euros(c); got != attendu {
			t.Errorf("Euros(%d) = %q, attendu %q", c, got, attendu)
		}
	}
}
