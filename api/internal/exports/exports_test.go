package exports

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/arysis/microdash/api/baremes"
	"github.com/arysis/microdash/api/internal/bareme"
	"github.com/arysis/microdash/api/internal/calc"
	"github.com/arysis/microdash/api/internal/store"
)

func jour(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func charger(t *testing.T) *bareme.Set {
	t.Helper()
	set, err := bareme.Load(baremes.FS)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

var saisies = []store.Transaction{
	{ID: 3, Type: "recette", Date: jour("2026-10-03"), Centimes: 240000, Categorie: "services_bic", Libelle: "Chantier", Tiers: "=HYPERLINK(\"x\")"},
	{ID: 2, Type: "depense", Date: jour("2026-10-02"), Centimes: 32000, Poste: "Matériel", Libelle: "Outillage; vis"},
	{ID: 1, Type: "recette", Date: jour("2026-10-02"), Centimes: 95050, Categorie: "services_bic", Libelle: "Installation"},
}

func TestCSV(t *testing.T) {
	set := charger(t)
	b, _ := set.Pour(2026)
	var buf bytes.Buffer
	if err := CSV(&buf, saisies, b); err != nil {
		t.Fatal(err)
	}
	attendu := "\xef\xbb\xbfDate;Type;Catégorie;Libellé;Client ou fournisseur;Montant (€)\r\n" +
		"02/10/2026;Recette;Prestations de services commerciales ou artisanales (BIC);Installation;;950,50\r\n" +
		"02/10/2026;Dépense;Matériel;\"Outillage; vis\";;-320,00\r\n" +
		"03/10/2026;Recette;Prestations de services commerciales ou artisanales (BIC);Chantier;\"'=HYPERLINK(\"\"x\"\")\";2400,00\r\n"
	if buf.String() != attendu {
		t.Errorf("CSV =\n%q\nattendu\n%q", buf.String(), attendu)
	}
}

func TestPDF(t *testing.T) {
	set := charger(t)
	b, _ := set.Pour(2026)
	for _, periodicite := range []string{"trimestrielle", "mensuelle"} {
		p := store.Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2024-03-01"), Periodicite: periodicite, VersementLiberatoire: periodicite == "mensuelle"}
		res, err := calc.Calculer(2026, p.VersCalc(), store.VersCalc(saisies), set)
		if err != nil {
			t.Fatal(err)
		}
		// Beaucoup de saisies, pour passer sur plusieurs pages.
		beaucoup := append([]store.Transaction(nil), saisies...)
		for i := 0; i < 80; i++ {
			beaucoup = append(beaucoup, store.Transaction{ID: int64(10 + i), Type: "recette", Date: jour("2026-03-15"), Centimes: 1000, Categorie: "services_bic", Libelle: strings.Repeat("Libellé très long ", 6)})
		}
		var buf bytes.Buffer
		err = PDF(&buf, Recapitulatif{Email: "moi@exemple.fr", Profil: p, Resultat: res, Saisies: beaucoup, Bareme: b, EditeLe: jour("2026-10-06")})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) || buf.Len() < 5000 {
			t.Errorf("%s : PDF invalide (%d octets)", periodicite, buf.Len())
		}
	}
}
