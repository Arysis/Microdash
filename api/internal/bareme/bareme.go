// Package bareme charge les taux, plafonds et règles annuelles de la micro-entreprise.
package bareme

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Date est une date YAML au format AAAA-MM-JJ.
type Date struct{ time.Time }

func (d *Date) UnmarshalYAML(n *yaml.Node) error {
	t, err := time.Parse("2006-01-02", n.Value)
	if err != nil {
		return fmt.Errorf("date invalide %q: %w", n.Value, err)
	}
	d.Time = t
	return nil
}

type TauxDate struct {
	Depuis Date    `yaml:"depuis"`
	Taux   float64 `yaml:"taux"`
}

type Franchise struct {
	Base   float64 `yaml:"base"`
	Majore float64 `yaml:"majore"`
}

type Categorie struct {
	Libelle              string     `yaml:"libelle" json:"libelle"`
	GroupePlafond        string     `yaml:"groupe_plafond" json:"groupe_plafond"`
	PlafondCA            float64    `yaml:"plafond_ca" json:"plafond_ca"`
	Abattement           float64    `yaml:"abattement" json:"abattement"`
	VersementLiberatoire float64    `yaml:"versement_liberatoire" json:"versement_liberatoire"`
	Cotisations          []TauxDate `yaml:"cotisations" json:"-"`
	TVAFranchise         Franchise  `yaml:"tva_franchise" json:"tva_franchise"`
}

type ACRE struct {
	DebutActiviteDepuis Date    `yaml:"debut_activite_depuis"`
	Reduction           float64 `yaml:"reduction"`
}

// Echeances regroupe les dates fixes de l'agenda qui ne viennent pas de l'URSSAF.
type Echeances struct {
	// CFE : date limite de paiement de la cotisation foncière des entreprises.
	CFE struct {
		Mois int `yaml:"mois"`
		Jour int `yaml:"jour"`
	} `yaml:"cfe"`
	// DeclarationRevenus : date limite de la déclaration en ligne de la zone la plus tôt ;
	// nil tant qu'elle n'est pas publiée.
	DeclarationRevenus *Date `yaml:"declaration_revenus"`
}

type Bareme struct {
	Annee             int                  `yaml:"annee"`
	Categories        map[string]Categorie `yaml:"categories"`
	PlafondMixte      float64              `yaml:"plafond_mixte"`
	ACRE              []ACRE               `yaml:"acre"`
	CFP               map[string]float64   `yaml:"cfp"`
	AbattementMinimum float64              `yaml:"abattement_minimum"`
	Echeances         Echeances            `yaml:"echeances"`
}

// TauxCotisation renvoie le taux plein de la catégorie en vigueur à la date donnée.
// Avant le premier taux du barème (barème d'une autre année), c'est ce premier taux qui s'applique.
func (c Categorie) TauxCotisation(d time.Time) float64 {
	if len(c.Cotisations) == 0 {
		return 0
	}
	taux := c.Cotisations[0].Taux
	for _, t := range c.Cotisations {
		if !d.Before(t.Depuis.Time) {
			taux = t.Taux
		}
	}
	return taux
}

// ReductionACRE renvoie la réduction ACRE applicable à une activité commencée à la date donnée.
func (b *Bareme) ReductionACRE(debut time.Time) float64 {
	r := 0.0
	for _, a := range b.ACRE {
		if !debut.Before(a.DebutActiviteDepuis.Time) {
			r = a.Reduction
		}
	}
	return r
}

// Set regroupe les barèmes de toutes les années disponibles.
type Set struct {
	parAnnee map[int]*Bareme
	annees   []int
}

// Load lit tous les fichiers <année>.yaml d'un système de fichiers.
func Load(fsys fs.FS) (*Set, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	s := &Set{parAnnee: map[int]*Bareme{}}
	for _, e := range entries {
		nom := e.Name()
		if e.IsDir() || path.Ext(nom) != ".yaml" {
			continue
		}
		annee, err := strconv.Atoi(strings.TrimSuffix(nom, ".yaml"))
		if err != nil {
			continue
		}
		data, err := fs.ReadFile(fsys, nom)
		if err != nil {
			return nil, err
		}
		var b Bareme
		if err := yaml.Unmarshal(data, &b); err != nil {
			return nil, fmt.Errorf("%s: %w", nom, err)
		}
		if b.Annee != annee {
			return nil, fmt.Errorf("%s: annee %d ne correspond pas au nom du fichier", nom, b.Annee)
		}
		for code, c := range b.Categories {
			if len(c.Cotisations) == 0 {
				return nil, fmt.Errorf("%s: catégorie %s sans taux de cotisation", nom, code)
			}
		}
		s.parAnnee[annee] = &b
		s.annees = append(s.annees, annee)
	}
	if len(s.annees) == 0 {
		return nil, fmt.Errorf("aucun barème trouvé")
	}
	sort.Ints(s.annees)
	return s, nil
}

// Pour renvoie le barème de l'année, ou à défaut le plus récent antérieur (ou le plus ancien).
// Le booléen indique si le barème exact de l'année existe.
func (s *Set) Pour(annee int) (*Bareme, bool) {
	if b, ok := s.parAnnee[annee]; ok {
		return b, true
	}
	choix := s.annees[0]
	for _, a := range s.annees {
		if a <= annee {
			choix = a
		}
	}
	return s.parAnnee[choix], false
}

// Annees liste les années disponibles, de la plus ancienne à la plus récente.
func (s *Set) Annees() []int { return append([]int(nil), s.annees...) }
