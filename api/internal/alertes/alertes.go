// Package alertes envoie chaque matin les rappels d'échéances et les alertes de plafonds par e-mail.
package alertes

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/arysis/microdash/api/internal/bareme"
	"github.com/arysis/microdash/api/internal/calc"
	"github.com/arysis/microdash/api/internal/store"
)

// Paris est le fuseau des dates limites et de l'heure d'envoi.
var Paris = func() *time.Location {
	l, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		panic(err)
	}
	return l
}()

// Aujourdhui renvoie la date du jour à Paris, à minuit UTC comme les dates du calcul.
func Aujourdhui(t time.Time) time.Time {
	a, m, j := t.In(Paris).Date()
	return time.Date(a, m, j, 0, 0, 0, 0, time.UTC)
}

// Source est ce que les alertes lisent et écrivent en base.
type Source interface {
	Transactions(ctx context.Context, userID int64, du, au time.Time) ([]store.Transaction, error)
	EcheancesFaites(ctx context.Context, userID int64) (map[string]time.Time, error)
	Destinataires(ctx context.Context) ([]store.Destinataire, error)
	ReserverAlerte(ctx context.Context, userID int64, cle string) (bool, error)
	AnnulerAlerte(ctx context.Context, userID int64, cle string) error
}

// Agenda calcule l'agenda d'une personne pour une année, à partir de ses saisies.
func Agenda(ctx context.Context, src Source, set *bareme.Set, userID int64, p store.Profil, annee int) ([]calc.Echeance, error) {
	txs, err := src.Transactions(ctx, userID, time.Date(annee-1, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(annee, 12, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		return nil, err
	}
	return calc.Agenda(annee, p.VersCalc(), p.Periodicite, store.VersCalc(txs), set)
}

// --- Lien de désinscription ---

// Signature signe les liens de désinscription, qui marchent sans connexion.
type Signature []byte

func (s Signature) mac(uid string) string {
	h := hmac.New(sha256.New, s)
	h.Write([]byte("desinscription:" + uid))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

// Jeton renvoie le jeton de désinscription d'un compte.
func (s Signature) Jeton(userID int64) string {
	uid := strconv.FormatInt(userID, 10)
	return uid + "." + s.mac(uid)
}

// Verifier renvoie le compte d'un jeton valide.
func (s Signature) Verifier(jeton string) (int64, bool) {
	uid, sig, ok := strings.Cut(jeton, ".")
	if !ok || len(s) == 0 || !hmac.Equal([]byte(sig), []byte(s.mac(uid))) {
		return 0, false
	}
	id, err := strconv.ParseInt(uid, 10, 64)
	return id, err == nil
}

// --- Choix des alertes ---

// Alerte est un e-mail à envoyer, identifié par une clé unique par compte.
type Alerte struct {
	Cle    string
	Sujet  string
	Corps  string   // texte avant le pied de l'e-mail
	Aussi  []string // clés à marquer comme envoyées en même temps (seuil 80 % quand 100 % part directement)
	Compte int64
}

var moisFR = []string{"janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"}
var joursFR = []string{"dimanche", "lundi", "mardi", "mercredi", "jeudi", "vendredi", "samedi"}

func dateLongue(t time.Time) string {
	j := strconv.Itoa(t.Day())
	if t.Day() == 1 {
		j = "1er"
	}
	return fmt.Sprintf("%s %s %s %d", joursFR[t.Weekday()], j, moisFR[t.Month()-1], t.Year())
}

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

func dans(jours int) string {
	switch jours {
	case 0:
		return "aujourd'hui"
	case 1:
		return "demain"
	}
	return fmt.Sprintf("dans %d jours", jours)
}

const (
	lienURSSAF = "https://www.autoentrepreneur.urssaf.fr"
	lienImpots = "https://www.impots.gouv.fr"
)

// Rappels choisit les rappels d'échéances du jour : un premier entre 7 et 2 jours avant,
// un second la veille ou le jour même. Les échéances cochées ou sans date n'en ont pas.
func Rappels(d store.Destinataire, agenda []calc.Echeance, faites map[string]time.Time, aujourdhui time.Time) []Alerte {
	var res []Alerte
	for _, e := range agenda {
		if e.Date == "" {
			continue
		}
		if _, ok := faites[e.Code]; ok {
			continue
		}
		switch e.Type {
		case "cfe":
			if !d.Preferences.CFE {
				continue
			}
		default:
			if !d.Preferences.Echeances {
				continue
			}
		}
		date, err := time.Parse("2006-01-02", e.Date)
		if err != nil {
			continue
		}
		jours := int(date.Sub(aujourdhui).Hours() / 24)
		var etape string
		switch {
		case jours >= 2 && jours <= 7:
			etape = "j7"
		case jours >= 0 && jours <= 1:
			etape = "j1"
		default:
			continue
		}
		res = append(res, Alerte{
			Cle: e.Code + ":" + etape, Compte: d.ID,
			Sujet: fmt.Sprintf("%s : %s", e.Libelle, dans(jours)),
			Corps: corpsEcheance(e, date, jours),
		})
	}
	return res
}

func corpsEcheance(e calc.Echeance, date time.Time, jours int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Bonjour,\n\n%s : à faire au plus tard le %s (%s).\n", e.Libelle, dateLongue(date), dans(jours))
	switch e.Type {
	case "urssaf":
		fmt.Fprintf(&b, "\nChiffre d'affaires à déclarer, selon tes saisies : %s\n", Euros(e.CA))
		fmt.Fprintf(&b, "Cotisations estimées : %s\n", Euros(e.APayer))
		if !e.BaremeExact {
			b.WriteString("Estimation faite avec les taux d'une autre année : vérifie le montant sur le site de l'URSSAF.\n")
		}
		if e.Note != "" {
			b.WriteString(e.Note + "\n")
		}
		fmt.Fprintf(&b, "\nDéclarer : %s\n", lienURSSAF)
	case "cfe":
		b.WriteString("\n" + e.Note + "\n")
		fmt.Fprintf(&b, "\nPayer : ton espace professionnel sur %s\n", lienImpots)
	default:
		if e.Note != "" {
			b.WriteString("\n" + e.Note + "\n")
		}
		fmt.Fprintf(&b, "\nDéclarer : %s\n", lienImpots)
	}
	return b.String()
}

func pourcent(r float64) string { return strconv.Itoa(int(math.Floor(r*100))) + " %" }

// Plafonds choisit les alertes de plafonds : à 80 %, puis au dépassement, une fois par an chacune.
func Plafonds(d store.Destinataire, r *calc.Resultat) []Alerte {
	var res []Alerte
	for _, p := range r.Plafonds {
		tva := strings.HasPrefix(p.Code, "tva_")
		if (tva && !d.Preferences.TVA) || (!tva && !d.Preferences.Plafond) {
			continue
		}
		base := fmt.Sprintf("plafond-%d-%s", r.Annee, p.Code)
		var b strings.Builder
		b.WriteString("Bonjour,\n\n")
		fmt.Fprintf(&b, "Selon tes saisies, ton chiffre d'affaires %d atteint %s sur un seuil de %s (%s).\n\n", r.Annee, Euros(p.CA), Euros(p.Plafond), pourcent(p.Ratio))
		a := Alerte{Compte: d.ID}
		switch {
		case tva && p.Majore > 0 && p.CA > p.Majore:
			a.Cle, a.Aussi = base+":majore", []string{base + ":80", base + ":100"}
			a.Sujet = "Seuil majoré de franchise de TVA dépassé"
			fmt.Fprintf(&b, "Tu as dépassé le seuil majoré de %s : la TVA est due dès le jour du dépassement.\n", Euros(p.Majore))
		case p.Niveau == "depasse":
			a.Cle, a.Aussi = base+":100", []string{base + ":80"}
			if tva {
				a.Sujet = "Seuil de franchise de TVA dépassé"
				b.WriteString("Tu as dépassé le seuil de franchise de TVA : tu deviens redevable de la TVA au 1er janvier suivant.")
				if p.Majore > 0 {
					fmt.Fprintf(&b, " Au-delà de %s, elle est due dès le jour du dépassement.", Euros(p.Majore))
				}
				b.WriteString("\n")
			} else {
				a.Sujet = "Plafond de la micro-entreprise dépassé"
				b.WriteString("Tu as dépassé le plafond de la micro-entreprise. Si cela se répète l'année suivante, tu sors du régime micro au 1er janvier d'après.\n")
			}
		case p.Niveau == "attention":
			a.Cle = base + ":80"
			if tva {
				a.Sujet = "Tu approches du seuil de franchise de TVA"
			} else {
				a.Sujet = "Tu approches du plafond de la micro-entreprise"
			}
			b.WriteString("Tu as passé 80 % du seuil. Garde un œil sur tes prochaines recettes.\n")
		default:
			continue
		}
		if tva {
			fmt.Fprintf(&b, "\nVérifie ta situation sur %s.\n", lienImpots)
		} else {
			fmt.Fprintf(&b, "\nVérifie ta situation sur %s.\n", lienURSSAF)
		}
		a.Corps = b.String()
		res = append(res, a)
	}
	return res
}

// --- Envoi ---

// Service relie la base, le calcul et l'envoi.
type Service struct {
	Source    Source
	Baremes   *bareme.Set
	Envoyeur  Envoyeur
	Signature Signature
	AppURL    string   // adresse publique du site, sans / final
	LimiterA  []string // si non vide, seules ces adresses reçoivent des e-mails
}

func (s *Service) autorise(email string) bool {
	if len(s.LimiterA) == 0 {
		return true
	}
	for _, a := range s.LimiterA {
		if strings.EqualFold(strings.TrimSpace(a), email) {
			return true
		}
	}
	return false
}

// pied renvoie la fin de l'e-mail et le lien de désinscription en un clic, qui vise l'API
// (les messageries l'appellent en POST, sans passer par la page).
func (s *Service) pied(userID int64) (texte, unClic string) {
	jeton := s.Signature.Jeton(userID)
	page := s.AppURL + "/desinscription?jeton=" + jeton
	texte = fmt.Sprintf("\nTon agenda : %s/agenda\n\n--\nTu reçois cet e-mail parce que les rappels sont activés dans ton compte Microdash. Pour ne plus en recevoir : %s\n", s.AppURL, page)
	return texte, s.AppURL + "/api/alertes/desinscription?jeton=" + jeton
}

// Tourner calcule et envoie les alertes du jour. Une alerte déjà partie ne repart jamais ;
// un envoi qui échoue est retenté au passage suivant.
func (s *Service) Tourner(ctx context.Context, maintenant time.Time) error {
	jour := Aujourdhui(maintenant)
	dests, err := s.Source.Destinataires(ctx)
	if err != nil {
		return err
	}
	envoyes := 0
	for _, d := range dests {
		if !s.autorise(d.Email) {
			continue
		}
		alertes, err := s.alertesDe(ctx, d, jour)
		if err != nil {
			slog.Error("alertes : calcul", "compte", d.ID, "err", err)
			continue
		}
		for _, a := range alertes {
			ok, err := s.Source.ReserverAlerte(ctx, d.ID, a.Cle)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			pied, lien := s.pied(d.ID)
			err = s.Envoyeur.Envoyer(ctx, Message{A: d.Email, Sujet: a.Sujet, Texte: a.Corps + pied, Desinscription: lien})
			if err != nil {
				slog.Error("alertes : envoi", "compte", d.ID, "cle", a.Cle, "err", err)
				if err := s.Source.AnnulerAlerte(ctx, d.ID, a.Cle); err != nil {
					return err
				}
				continue
			}
			for _, c := range a.Aussi {
				if _, err := s.Source.ReserverAlerte(ctx, d.ID, c); err != nil {
					return err
				}
			}
			envoyes++
		}
	}
	slog.Info("alertes envoyées", "nombre", envoyes)
	return nil
}

func (s *Service) alertesDe(ctx context.Context, d store.Destinataire, jour time.Time) ([]Alerte, error) {
	var res []Alerte
	if d.Preferences.Echeances || d.Preferences.CFE {
		agenda, err := Agenda(ctx, s.Source, s.Baremes, d.ID, d.Profil, jour.Year())
		if err != nil {
			return nil, err
		}
		faites, err := s.Source.EcheancesFaites(ctx, d.ID)
		if err != nil {
			return nil, err
		}
		res = append(res, Rappels(d, agenda, faites, jour)...)
	}
	if (d.Preferences.Plafond || d.Preferences.TVA) && !d.Profil.DebutActivite.After(jour) {
		annee := jour.Year()
		txs, err := s.Source.Transactions(ctx, d.ID, time.Date(annee, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(annee, 12, 31, 0, 0, 0, 0, time.UTC))
		if err != nil {
			return nil, err
		}
		r, err := calc.Calculer(annee, d.Profil.VersCalc(), store.VersCalc(txs), s.Baremes)
		if err != nil {
			return nil, err
		}
		res = append(res, Plafonds(d, r)...)
	}
	return res, nil
}

// ProchainPassage renvoie le prochain 8 h à Paris après t.
func ProchainPassage(t time.Time) time.Time {
	p := t.In(Paris)
	h := time.Date(p.Year(), p.Month(), p.Day(), 8, 0, 0, 0, Paris)
	if !h.After(t) {
		h = time.Date(p.Year(), p.Month(), p.Day()+1, 8, 0, 0, 0, Paris)
	}
	return h
}

// Planifier lance un passage au démarrage s'il est déjà 8 h passées, puis chaque jour à 8 h (Paris).
func (s *Service) Planifier(ctx context.Context) {
	if t := time.Now().In(Paris); t.Hour() >= 8 {
		if err := s.Tourner(ctx, time.Now()); err != nil {
			slog.Error("alertes", "err", err)
		}
	}
	for {
		attente := time.Until(ProchainPassage(time.Now()))
		select {
		case <-ctx.Done():
			return
		case <-time.After(attente):
			if err := s.Tourner(ctx, time.Now()); err != nil {
				slog.Error("alertes", "err", err)
			}
		}
	}
}
