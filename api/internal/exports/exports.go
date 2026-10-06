// Package exports produit les saisies en CSV et le récapitulatif annuel en PDF.
package exports

import (
	"embed"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/arysis/microdash/api/internal/bareme"
	"github.com/arysis/microdash/api/internal/calc"
	"github.com/arysis/microdash/api/internal/store"
)

// Polices IBM Plex Sans (licence SIL OFL, voir polices/OFL.txt), comme dans l'app.
//
//go:embed polices/*.ttf
var polices embed.FS

// montantCSV écrit des centimes en euros avec une virgule, sans séparateur de milliers,
// pour que les tableurs français le lisent comme un nombre.
func montantCSV(c int64) string {
	signe := ""
	if c < 0 {
		signe, c = "-", -c
	}
	return fmt.Sprintf("%s%d,%02d", signe, c/100, c%100)
}

// texteCSV neutralise un texte qu'un tableur prendrait pour une formule.
func texteCSV(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

func libelleCategorie(b *bareme.Bareme, t store.Transaction) string {
	if t.Type == calc.Depense {
		return t.Poste
	}
	if c, ok := b.Categories[t.Categorie]; ok {
		return c.Libelle
	}
	return t.Categorie
}

// trierParDate range les saisies de la plus ancienne à la plus récente.
func trierParDate(txs []store.Transaction) []store.Transaction {
	res := append([]store.Transaction(nil), txs...)
	sort.SliceStable(res, func(i, j int) bool {
		if !res[i].Date.Equal(res[j].Date) {
			return res[i].Date.Before(res[j].Date)
		}
		return res[i].ID < res[j].ID
	})
	return res
}

// CSV écrit les saisies au format attendu par Excel et LibreOffice en français :
// UTF-8 avec BOM, point-virgule, virgule décimale. Les dépenses sont négatives.
func CSV(w io.Writer, txs []store.Transaction, b *bareme.Bareme) error {
	if _, err := io.WriteString(w, "\xef\xbb\xbf"); err != nil {
		return err
	}
	c := csv.NewWriter(w)
	c.Comma = ';'
	c.UseCRLF = true
	if err := c.Write([]string{"Date", "Type", "Catégorie", "Libellé", "Client ou fournisseur", "Montant (€)"}); err != nil {
		return err
	}
	for _, t := range trierParDate(txs) {
		typ, montant := "Recette", t.Centimes
		if t.Type == calc.Depense {
			typ, montant = "Dépense", -t.Centimes
		}
		if err := c.Write([]string{
			t.Date.Format("02/01/2006"), typ, texteCSV(libelleCategorie(b, t)),
			texteCSV(t.Libelle), texteCSV(t.Tiers), montantCSV(montant),
		}); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}

// --- PDF ---

// Recapitulatif rassemble ce que contient le PDF d'une année.
type Recapitulatif struct {
	Email    string
	Profil   store.Profil
	Resultat *calc.Resultat
	Saisies  []store.Transaction
	Bareme   *bareme.Bareme
	EditeLe  time.Time
}

type couleur struct{ r, g, b int }

var (
	encre    = couleur{28, 25, 23}
	pierre   = couleur{87, 83, 78}
	lin      = couleur{231, 229, 228}
	sarcelle = couleur{15, 94, 89}
	safran   = couleur{194, 65, 12}
	brique   = couleur{185, 28, 28}
)

var moisFR = []string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}

func dateFR(t time.Time) string {
	j := strconv.Itoa(t.Day())
	if t.Day() == 1 {
		j = "1er"
	}
	return fmt.Sprintf("%s %s %d", j, moisFR[t.Month()-1], t.Year())
}

type document struct {
	*fpdf.Fpdf
	largeur float64 // largeur utile entre les marges
}

const marge = 18.0

func (d *document) couleurTexte(c couleur) { d.SetTextColor(c.r, c.g, c.b) }

func (d *document) police(gras bool, taille float64) {
	style := ""
	if gras {
		style = "B"
	}
	d.SetFont("Plex", style, taille)
}

func (d *document) titreSection(t string) {
	if d.GetY() > 250 {
		d.AddPage()
	}
	d.Ln(6)
	d.police(true, 13)
	d.couleurTexte(encre)
	d.CellFormat(0, 8, t, "", 1, "L", false, 0, "")
	d.Ln(1)
}

func (d *document) filet() {
	d.SetDrawColor(lin.r, lin.g, lin.b)
	d.SetLineWidth(0.2)
	y := d.GetY()
	d.Line(marge, y, marge+d.largeur, y)
}

// ligne écrit une rangée de cellules ; les largeurs sont en fractions de la largeur utile.
func (d *document) ligne(cellules []string, largeurs []float64, alignements string, gras bool, c couleur) {
	if d.GetY() > 270 {
		d.AddPage()
	}
	d.police(gras, 9.5)
	d.couleurTexte(c)
	for i, txt := range cellules {
		w := largeurs[i] * d.largeur
		d.CellFormat(w, 6.5, d.couper(txt, w-1.5), "", 0, string(alignements[i]), false, 0, "")
	}
	d.Ln(-1)
	d.filet()
}

// couper raccourcit un texte trop long pour sa cellule.
func (d *document) couper(s string, w float64) string {
	if d.GetStringWidth(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && d.GetStringWidth(string(r)+"…") > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

func (d *document) paire(libelle, valeur string, gras bool) {
	d.ligne([]string{libelle, valeur}, []float64{0.7, 0.3}, "LR", gras, encre)
}

// euros formate un montant pour le PDF : la police embarquée n'a pas l'espace fine
// insécable des milliers, remplacée par une espace insécable.
func euros(c int64) string { return strings.ReplaceAll(calc.Euros(c), "\u202f", "\u00a0") }

func ouiNon(b bool) string {
	if b {
		return "oui"
	}
	return "non"
}

// PDF écrit le récapitulatif de l'année : profil, totaux, détail par période, plafonds et saisies.
func PDF(w io.Writer, r Recapitulatif) error {
	f := fpdf.New("P", "mm", "A4", "")
	for nom, style := range map[string]string{"IBMPlexSans-Regulier.ttf": "", "IBMPlexSans-Gras.ttf": "B"} {
		octets, err := polices.ReadFile("polices/" + nom)
		if err != nil {
			return err
		}
		f.AddUTF8FontFromBytes("Plex", style, octets)
	}
	res := r.Resultat
	f.SetTitle(fmt.Sprintf("Récapitulatif %d", res.Annee), true)
	f.SetCreator("Microdash", true)
	f.SetMargins(marge, marge, marge)
	f.SetAutoPageBreak(true, 22)
	f.AliasNbPages("{nb}")
	d := &document{Fpdf: f, largeur: 210 - 2*marge}
	f.SetFooterFunc(func() {
		f.SetY(-16)
		d.police(false, 8)
		d.couleurTexte(pierre)
		f.CellFormat(d.largeur*0.8, 5, "Estimations de Microdash à partir de tes saisies. Seule ta déclaration URSSAF fait foi.", "", 0, "L", false, 0, "")
		f.CellFormat(d.largeur*0.2, 5, fmt.Sprintf("Page %d sur {nb}", f.PageNo()), "", 0, "R", false, 0, "")
	})
	f.AddPage()

	// En-tête
	d.police(true, 10)
	d.couleurTexte(safran)
	f.CellFormat(0, 5, "Microdash", "", 1, "L", false, 0, "")
	d.police(true, 22)
	d.couleurTexte(encre)
	f.CellFormat(0, 12, fmt.Sprintf("Récapitulatif %d", res.Annee), "", 1, "L", false, 0, "")
	d.police(false, 10)
	d.couleurTexte(pierre)
	f.CellFormat(0, 6, fmt.Sprintf("%s · édité le %s", r.Email, dateFR(r.EditeLe)), "", 1, "L", false, 0, "")
	if !res.BaremeExact {
		f.Ln(2)
		d.couleurTexte(safran)
		f.MultiCell(0, 5, fmt.Sprintf("Pas encore de barème %d : les calculs utilisent les taux %d.", res.Annee, res.BaremeAnnee), "", "L", false)
	}

	// Activité
	d.titreSection("Ton activité")
	p := r.Profil
	cat := p.Categorie
	if c, ok := r.Bareme.Categories[p.Categorie]; ok {
		cat = c.Libelle
	}
	d.ligne([]string{"Catégorie", cat}, []float64{0.3, 0.7}, "LR", false, encre)
	if p.CategorieSecondaire != "" {
		sec := p.CategorieSecondaire
		if c, ok := r.Bareme.Categories[sec]; ok {
			sec = c.Libelle
		}
		d.ligne([]string{"Activité secondaire", sec}, []float64{0.3, 0.7}, "LR", false, encre)
	}
	d.ligne([]string{"Début d'activité", dateFR(p.DebutActivite)}, []float64{0.3, 0.7}, "LR", false, encre)
	d.ligne([]string{"Déclaration URSSAF", p.Periodicite}, []float64{0.3, 0.7}, "LR", false, encre)
	acre := ouiNon(p.ACRE)
	if res.FinACRE != "" {
		if fin, err := time.Parse("2006-01-02", res.FinACRE); err == nil {
			acre = "oui, jusqu'au " + dateFR(fin)
		}
	}
	d.ligne([]string{"ACRE", acre}, []float64{0.3, 0.7}, "LR", false, encre)
	d.ligne([]string{"Versement libératoire", ouiNon(p.VersementLiberatoire)}, []float64{0.3, 0.7}, "LR", false, encre)

	// Totaux
	t := res.Total
	d.titreSection(fmt.Sprintf("Totaux %d", res.Annee))
	d.paire("Chiffre d'affaires encaissé", euros(t.CA), false)
	d.paire("Cotisations sociales", "− "+euros(t.Cotisations), false)
	d.paire("Formation professionnelle (CFP)", "− "+euros(t.CFP), false)
	if p.VersementLiberatoire {
		d.paire("Impôt (versement libératoire)", "− "+euros(t.ImpotVL), false)
	}
	d.paire("Dépenses", "− "+euros(t.Depenses), false)
	d.paire("Revenu net estimé", euros(t.Net), true)
	if !p.VersementLiberatoire {
		d.paire("Revenu imposable de l'activité (après abattement)", euros(res.RevenuImposable), false)
	}

	// Détail par période, selon la périodicité de déclaration
	trimestres := []string{"1er trimestre", "2e trimestre", "3e trimestre", "4e trimestre"}
	periodes, titre, nom := res.Trimestres, "Par trimestre", func(i int, _ calc.Periode) string { return trimestres[i] }
	if p.Periodicite == calc.Mensuelle {
		periodes, titre, nom = res.Mois, "Par mois", func(_ int, pe calc.Periode) string { return pe.Libelle }
	}
	d.titreSection(titre)
	largeurs := []float64{0.24, 0.19, 0.19, 0.19, 0.19}
	d.ligne([]string{"Période", "Chiffre d'affaires", "Cotisations et CFP", "Dépenses", "Net"}, largeurs, "LRRRR", true, pierre)
	for i, pe := range periodes {
		d.ligne([]string{nom(i, pe), euros(pe.CA), euros(pe.Cotisations + pe.CFP + pe.ImpotVL), euros(pe.Depenses), euros(pe.Net)}, largeurs, "LRRRR", false, encre)
	}
	d.ligne([]string{"Année", euros(t.CA), euros(t.Cotisations + t.CFP + t.ImpotVL), euros(t.Depenses), euros(t.Net)}, largeurs, "LRRRR", true, encre)
	if p.VersementLiberatoire {
		d.police(false, 8.5)
		d.couleurTexte(pierre)
		f.CellFormat(0, 6, "La colonne Cotisations et CFP inclut l'impôt du versement libératoire.", "", 1, "L", false, 0, "")
	}

	// Plafonds
	d.titreSection(fmt.Sprintf("Plafonds %d", res.Annee))
	lp := []float64{0.46, 0.2, 0.2, 0.14}
	d.ligne([]string{"Plafond", "Chiffre d'affaires", "Seuil", "Atteint"}, lp, "LRRR", true, pierre)
	for _, pl := range res.Plafonds {
		c := encre
		switch pl.Niveau {
		case "attention":
			c = safran
		case "depasse":
			c = brique
		}
		d.ligne([]string{pl.Libelle, euros(pl.CA), euros(pl.Plafond), fmt.Sprintf("%d %%", int(math.Round(pl.Ratio*100)))}, lp, "LRRR", false, c)
	}

	// Saisies
	saisies := trierParDate(r.Saisies)
	d.titreSection(fmt.Sprintf("Saisies %d (%d)", res.Annee, len(saisies)))
	if len(saisies) == 0 {
		d.police(false, 9.5)
		d.couleurTexte(pierre)
		f.CellFormat(0, 6, "Aucune saisie cette année.", "", 1, "L", false, 0, "")
	} else {
		ls := []float64{0.14, 0.34, 0.32, 0.2}
		d.ligne([]string{"Date", "Libellé", "Catégorie", "Montant"}, ls, "LLLR", true, pierre)
		for _, s := range saisies {
			lib := s.Libelle
			if s.Tiers != "" {
				if lib != "" {
					lib += " · "
				}
				lib += s.Tiers
			}
			if lib == "" {
				lib = map[bool]string{true: "Dépense", false: "Recette"}[s.Type == calc.Depense]
			}
			montant, c := "+ "+euros(s.Centimes), sarcelle
			if s.Type == calc.Depense {
				montant, c = "− "+euros(s.Centimes), encre
			}
			d.ligne([]string{s.Date.Format("02/01/2006"), lib, libelleCategorie(r.Bareme, s), montant}, ls, "LLLR", false, c)
		}
	}
	return f.Output(w)
}
