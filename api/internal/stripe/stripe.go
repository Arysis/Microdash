// Package stripe lit, avec une clé restreinte en lecture seule, les abonnements (MRR) et les
// encaissements d'un compte Stripe. Aucun appel n'écrit dans le compte.
package stripe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Version d'API figée : la forme des objets lus ici ne change pas quand Stripe publie une version.
const Version = "2024-06-20"

// pagesMax borne une lecture (100 objets par page) pour qu'elle reste rapide.
const pagesMax = 50

type Client struct {
	Cle  string
	URL  string // vide : https://api.stripe.com
	HTTP *http.Client
}

// ErreurStripe est une erreur renvoyée par Stripe, avec son message (il nomme la permission
// manquante quand la clé restreinte n'a pas le droit de lire une ressource).
type ErreurStripe struct {
	Statut  int
	Message string
}

func (e *ErreurStripe) Error() string { return fmt.Sprintf("Stripe (%d) : %s", e.Statut, e.Message) }

// VerifierCle refuse tout ce qui n'est pas une clé restreinte : une clé secrète (sk_) donne
// tous les droits sur le compte, Microdash n'en a pas besoin.
func VerifierCle(cle string) (mode string, err error) {
	switch {
	case strings.HasPrefix(cle, "rk_live_"):
		return "live", nil
	case strings.HasPrefix(cle, "rk_test_"):
		return "test", nil
	case strings.HasPrefix(cle, "sk_"):
		return "", errors.New("c'est une clé secrète, qui donne tous les droits sur ton compte Stripe : crée plutôt une clé restreinte en lecture seule (elle commence par rk_)")
	case strings.HasPrefix(cle, "pk_"):
		return "", errors.New("c'est une clé publiable : il faut une clé restreinte en lecture seule (elle commence par rk_)")
	}
	return "", errors.New("clé non reconnue : une clé restreinte Stripe commence par rk_live_ ou rk_test_")
}

func (c *Client) get(ctx context.Context, chemin string, params url.Values, v any) error {
	base := c.URL
	if base == "" {
		base = "https://api.stripe.com"
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	u := base + chemin
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Cle)
	req.Header.Set("Stripe-Version", Version)
	rep, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Stripe injoignable : %w", err)
	}
	defer rep.Body.Close()
	corps, err := io.ReadAll(io.LimitReader(rep.Body, 32<<20))
	if err != nil {
		return err
	}
	if rep.StatusCode != http.StatusOK {
		var e struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(corps, &e)
		if e.Error.Message == "" {
			e.Error.Message = rep.Status
		}
		return &ErreurStripe{Statut: rep.StatusCode, Message: e.Error.Message}
	}
	return json.Unmarshal(corps, v)
}

// lister parcourt toutes les pages d'une liste Stripe ; tronque indique qu'il en restait.
func lister[T any](ctx context.Context, c *Client, chemin string, params url.Values, id func(T) string) (res []T, tronque bool, err error) {
	params.Set("limit", "100")
	for page := 0; page < pagesMax; page++ {
		var l struct {
			Data    []T  `json:"data"`
			HasMore bool `json:"has_more"`
		}
		if err := c.get(ctx, chemin, params, &l); err != nil {
			return nil, false, err
		}
		res = append(res, l.Data...)
		if !l.HasMore || len(l.Data) == 0 {
			return res, false, nil
		}
		params.Set("starting_after", id(l.Data[len(l.Data)-1]))
	}
	return res, true, nil
}

// --- Objets lus (version 2024-06-20) ---

type Compte struct {
	ID              string `json:"id"`
	Email           string `json:"email"`
	BusinessProfile struct {
		Name string `json:"name"`
	} `json:"business_profile"`
	Settings struct {
		Dashboard struct {
			DisplayName string `json:"display_name"`
		} `json:"dashboard"`
	} `json:"settings"`
}

// Nom renvoie le nom le plus parlant du compte.
func (c Compte) Nom() string {
	for _, n := range []string{c.Settings.Dashboard.DisplayName, c.BusinessProfile.Name, c.Email} {
		if n != "" {
			return n
		}
	}
	return ""
}

type Coupon struct {
	PercentOff *float64 `json:"percent_off"`
	AmountOff  *int64   `json:"amount_off"`
	Currency   string   `json:"currency"`
}

type Remise struct {
	Coupon Coupon `json:"coupon"`
	End    *int64 `json:"end"`
}

type Abonnement struct {
	ID        string   `json:"id"`
	Status    string   `json:"status"`
	Currency  string   `json:"currency"`
	Discount  *Remise  `json:"discount"`
	Discounts []string `json:"discounts"`
	Items     struct {
		Data []struct {
			Quantity int64 `json:"quantity"`
			Price    struct {
				UnitAmount *int64 `json:"unit_amount"`
				Currency   string `json:"currency"`
				Recurring  *struct {
					Interval      string `json:"interval"`
					IntervalCount int64  `json:"interval_count"`
					UsageType     string `json:"usage_type"`
				} `json:"recurring"`
			} `json:"price"`
		} `json:"data"`
	} `json:"items"`
}

type Mouvement struct {
	ID                string `json:"id"`
	Amount            int64  `json:"amount"`
	Fee               int64  `json:"fee"`
	Currency          string `json:"currency"`
	Created           int64  `json:"created"`
	ReportingCategory string `json:"reporting_category"`
}

// --- Calcul du MRR ---

// parMois ramène un montant facturé tous les n intervalles à un montant mensuel.
func parMois(montant float64, intervalle string, n int64) (float64, bool) {
	if n <= 0 {
		n = 1
	}
	f := 0.0
	switch intervalle {
	case "day":
		f = 365.0 / 12
	case "week":
		f = 52.0 / 12
	case "month":
		f = 1
	case "year":
		f = 1.0 / 12
	default:
		return 0, false
	}
	return montant * f / float64(n), true
}

// MRR regroupe le revenu mensuel récurrent par devise, en centimes.
type MRR struct {
	ParDevise   map[string]int64 `json:"par_devise"`
	Actifs      int              `json:"actifs"`       // abonnements actifs ou en retard de paiement
	EnEssai     int              `json:"en_essai"`     // période d'essai, pas encore comptés
	NonCalcules int              `json:"non_calcules"` // tarifs à l'usage ou par paliers, pas comptés
	Remises     int              `json:"remises"`      // abonnements avec plusieurs remises : seule la première est déduite
}

// CalculerMRR additionne, pour les abonnements actifs ou en retard de paiement, le prix de chaque
// article × quantité ramené au mois, moins la remise de l'abonnement si elle court encore.
func CalculerMRR(abos []Abonnement, maintenant time.Time) MRR {
	m := MRR{ParDevise: map[string]int64{}}
	for _, a := range abos {
		switch a.Status {
		case "trialing":
			m.EnEssai++
			continue
		case "active", "past_due":
		default:
			continue
		}
		m.Actifs++
		total, intervalle, nombre, calcule := 0.0, "", int64(1), true
		for _, it := range a.Items.Data {
			p := it.Price
			if p.Recurring == nil || p.UnitAmount == nil || p.Recurring.UsageType == "metered" {
				calcule = false
				continue
			}
			mensuel, ok := parMois(float64(*p.UnitAmount*max(it.Quantity, 1)), p.Recurring.Interval, p.Recurring.IntervalCount)
			if !ok {
				calcule = false
				continue
			}
			total += mensuel
			intervalle, nombre = p.Recurring.Interval, p.Recurring.IntervalCount
		}
		if !calcule {
			m.NonCalcules++
		}
		if r := a.Discount; r != nil && (r.End == nil || time.Unix(*r.End, 0).After(maintenant)) {
			switch {
			case r.Coupon.PercentOff != nil:
				total *= 1 - *r.Coupon.PercentOff/100
			case r.Coupon.AmountOff != nil && intervalle != "":
				remise, _ := parMois(float64(*r.Coupon.AmountOff), intervalle, nombre)
				total = math.Max(0, total-remise)
			}
		}
		if len(a.Discounts) > 1 {
			m.Remises++
		}
		m.ParDevise[strings.ToLower(a.Currency)] += int64(math.Round(total))
	}
	return m
}

// --- Encaissements par mois ---

// MoisStripe résume les mouvements d'un mois civil (heure de Paris), en centimes d'euro.
type MoisStripe struct {
	Mois       string `json:"mois"` // AAAA-MM
	Encaisse   int64  `json:"encaisse"`
	Rembourse  int64  `json:"rembourse"`
	Frais      int64  `json:"frais"`
	Litiges    int64  `json:"litiges"`
	Mouvements int    `json:"mouvements"`
}

// ParMois regroupe les mouvements en euros : paiements reçus, remboursements, frais Stripe et litiges.
// Les virements vers la banque (payouts) ne sont pas des recettes et sont ignorés.
func ParMois(mvts []Mouvement, fuseau *time.Location) []MoisStripe {
	idx := map[string]*MoisStripe{}
	var cles []string
	for _, mv := range mvts {
		if strings.ToLower(mv.Currency) != "eur" {
			continue
		}
		cle := time.Unix(mv.Created, 0).In(fuseau).Format("2006-01")
		m := idx[cle]
		if m == nil {
			m = &MoisStripe{Mois: cle}
			idx[cle] = m
			cles = append(cles, cle)
		}
		switch mv.ReportingCategory {
		case "charge":
			m.Encaisse += mv.Amount
		case "refund":
			m.Rembourse += -mv.Amount
		case "fee":
			m.Frais += -mv.Amount
		case "dispute":
			m.Litiges += -mv.Amount
		default:
			continue
		}
		m.Frais += mv.Fee
		m.Mouvements++
	}
	sort.Strings(cles)
	res := make([]MoisStripe, 0, len(cles))
	for _, c := range cles {
		res = append(res, *idx[c])
	}
	return res
}

// --- Lecture complète ---

// Lecture est ce que Microdash garde d'un compte Stripe entre deux lectures.
type Lecture struct {
	CompteID  string       `json:"compte_id"`
	CompteNom string       `json:"compte_nom"`
	MRR       MRR          `json:"mrr"`
	Mois      []MoisStripe `json:"mois"`
	Tronque   bool         `json:"tronque"`
}

// Lire interroge le compte : identité (si la clé y a droit), abonnements et 12 mois de mouvements.
func (c *Client) Lire(ctx context.Context, maintenant time.Time, fuseau *time.Location) (*Lecture, error) {
	l := &Lecture{}
	var compte Compte
	if err := c.get(ctx, "/v1/account", nil, &compte); err == nil {
		l.CompteID, l.CompteNom = compte.ID, compte.Nom()
	}
	p := url.Values{}
	p.Set("status", "all")
	abos, tronque1, err := lister(ctx, c, "/v1/subscriptions", p, func(a Abonnement) string { return a.ID })
	if err != nil {
		return nil, err
	}
	l.MRR = CalculerMRR(abos, maintenant)

	t := maintenant.In(fuseau)
	debut := time.Date(t.Year(), t.Month()-11, 1, 0, 0, 0, 0, fuseau)
	p = url.Values{}
	p.Set("created[gte]", strconv.FormatInt(debut.Unix(), 10))
	mvts, tronque2, err := lister(ctx, c, "/v1/balance_transactions", p, func(m Mouvement) string { return m.ID })
	if err != nil {
		return nil, err
	}
	l.Mois = ParMois(mvts, fuseau)
	l.Tronque = tronque1 || tronque2
	return l, nil
}
