package calc

import (
	"fmt"
	"strconv"
	"strings"
)

// Euros formate des centimes à la française : 1 234,56 €.
func Euros(c int64) string {
	signe := ""
	if c < 0 {
		signe, c = "-", -c
	}
	ent := strconv.FormatInt(c/100, 10)
	var b strings.Builder
	for i, r := range ent {
		if i > 0 && (len(ent)-i)%3 == 0 {
			b.WriteRune(' ')
		}
		b.WriteRune(r)
	}
	return fmt.Sprintf("%s%s,%02d €", signe, b.String(), c%100)
}
