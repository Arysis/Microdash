package chiffre

import (
	"bytes"
	"strings"
	"testing"
)

func TestChiffre(t *testing.T) {
	c, err := Depuis(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	ch, err := c.Chiffrer([]byte("rk_live_secret"), "compte:1")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ch, []byte("rk_live")) {
		t.Fatal("texte clair visible")
	}
	if clair, err := c.Dechiffrer(ch, "compte:1"); err != nil || string(clair) != "rk_live_secret" {
		t.Fatalf("%q %v", clair, err)
	}
	if _, err := c.Dechiffrer(ch, "compte:2"); err == nil {
		t.Error("déchiffré avec le contexte d'un autre compte")
	}
	autre, _ := Depuis(strings.Repeat("cd", 32))
	if _, err := autre.Dechiffrer(ch, "compte:1"); err == nil {
		t.Error("déchiffré avec une autre clé")
	}
	for _, mauvaise := range []string{"", "abc", strings.Repeat("zz", 32), strings.Repeat("ab", 16)} {
		if _, err := Depuis(mauvaise); err == nil {
			t.Errorf("clé %q acceptée", mauvaise)
		}
	}
}
