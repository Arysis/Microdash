// Package qonto lit, avec la clé API d'une organisation Qonto, ses comptes et ses opérations.
// Aucun appel n'écrit dans le compte.
package qonto

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// pagesMax borne une lecture (100 opérations par page) pour qu'elle reste rapide.
const pagesMax = 50

type Client struct {
	Identifiant string // ex. mon-entreprise-1234
	Cle         string // clé secrète
	URL         string // vide : https://thirdparty.qonto.com
	HTTP        *http.Client
}

// ErreurQonto est une erreur renvoyée par Qonto, avec son message.
type ErreurQonto struct {
	Statut  int
	Message string
}

func (e *ErreurQonto) Error() string { return fmt.Sprintf("Qonto (%d) : %s", e.Statut, e.Message) }

// VerifierCle contrôle la forme de l'identifiant et de la clé avant tout appel.
func VerifierCle(identifiant, cle string) error {
	switch {
	case identifiant == "" || cle == "":
		return errors.New("colle l'identifiant et la clé secrète affichés par Qonto")
	case strings.ContainsAny(identifiant, " :/") || strings.ContainsAny(cle, " :/"):
		return errors.New("identifiant ou clé non reconnus : copie-les tels quels depuis Qonto, sans espace")
	case len(cle) < 8:
		return errors.New("clé trop courte : copie la clé secrète entière depuis Qonto")
	}
	return nil
}

func (c *Client) get(ctx context.Context, chemin string, params url.Values, v any) error {
	base := c.URL
	if base == "" {
		base = "https://thirdparty.qonto.com"
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
	req.Header.Set("Authorization", c.Identifiant+":"+c.Cle)
	req.Header.Set("Accept", "application/json")
	rep, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Qonto injoignable : %w", err)
	}
	defer rep.Body.Close()
	corps, err := io.ReadAll(io.LimitReader(rep.Body, 32<<20))
	if err != nil {
		return err
	}
	if rep.StatusCode != http.StatusOK {
		var e struct {
			Message string `json:"message"`
			Errors  []struct {
				Detail string `json:"detail"`
				Code   string `json:"code"`
			} `json:"errors"`
		}
		_ = json.Unmarshal(corps, &e)
		msg := e.Message
		if msg == "" && len(e.Errors) > 0 {
			msg = e.Errors[0].Detail
			if msg == "" {
				msg = e.Errors[0].Code
			}
		}
		if rep.StatusCode == http.StatusUnauthorized {
			msg = "identifiant ou clé refusés"
		}
		if msg == "" {
			msg = rep.Status
		}
		return &ErreurQonto{Statut: rep.StatusCode, Message: msg}
	}
	return json.Unmarshal(corps, v)
}

// --- Objets lus ---

type CompteBancaire struct {
	ID        string `json:"id"`
	Slug      string `json:"slug"`
	IBAN      string `json:"iban"`
	Nom       string `json:"name"`
	Statut    string `json:"status"`
	Devise    string `json:"currency"`
	Solde     int64  `json:"balance_cents"`
	Principal bool   `json:"main"`
	Externe   bool   `json:"is_external_account"`
}

type Organisation struct {
	Slug     string           `json:"slug"`
	Nom      string           `json:"name"`
	NomLegal string           `json:"legal_name"`
	Comptes  []CompteBancaire `json:"bank_accounts"`
}

// Libelle renvoie le nom le plus parlant de l'organisation.
func (o Organisation) Libelle() string {
	for _, n := range []string{o.NomLegal, o.Nom, o.Slug} {
		if n != "" {
			return n
		}
	}
	return ""
}

// ComptesSuivis renvoie les comptes Qonto ouverts en euros (pas les comptes d'autres banques).
func (o Organisation) ComptesSuivis() []CompteBancaire {
	var res []CompteBancaire
	for _, c := range o.Comptes {
		if c.Externe || c.Statut == "closed" || (c.Devise != "" && !strings.EqualFold(c.Devise, "EUR")) {
			continue
		}
		res = append(res, c)
	}
	return res
}

func (c *Client) Organisation(ctx context.Context) (*Organisation, error) {
	var r struct {
		Organisation Organisation `json:"organization"`
	}
	if err := c.get(ctx, "/v2/organization", nil, &r); err != nil {
		return nil, err
	}
	return &r.Organisation, nil
}

type Operation struct {
	ID           string     `json:"id"`
	Montant      int64      `json:"amount_cents"`
	Devise       string     `json:"currency"`
	Sens         string     `json:"side"` // credit ou debit
	Type         string     `json:"operation_type"`
	Libelle      string     `json:"label"`
	Contrepartie string     `json:"clean_counterparty_name"`
	Reference    string     `json:"reference"`
	Note         string     `json:"note"`
	Statut       string     `json:"status"`
	RegleLe      *time.Time `json:"settled_at"`
	Virement     *struct {
		IBAN string `json:"counterparty_account_number"`
	} `json:"transfer"`
}

// Nom renvoie ce qui désigne le mieux l'autre partie de l'opération.
func (o Operation) Nom() string {
	if s := strings.TrimSpace(o.Contrepartie); s != "" {
		return s
	}
	return strings.TrimSpace(o.Libelle)
}

// Operations lit les opérations réglées d'un compte depuis une date de règlement, et seulement
// celles modifiées depuis majDepuis quand elle n'est pas nulle. tronque : il en restait.
func (c *Client) Operations(ctx context.Context, compteID string, depuis time.Time, majDepuis *time.Time) (res []Operation, tronque bool, err error) {
	params := url.Values{}
	params.Set("bank_account_id", compteID)
	params.Add("status[]", "completed")
	params.Set("settled_at_from", depuis.UTC().Format(time.RFC3339))
	if majDepuis != nil {
		params.Set("updated_at_from", majDepuis.UTC().Format(time.RFC3339))
	}
	params.Add("includes[]", "transfer")
	params.Set("sort_by", "settled_at:asc")
	params.Set("per_page", "100")
	for page := 1; page <= pagesMax; page++ {
		params.Set("current_page", strconv.Itoa(page))
		var l struct {
			Operations []Operation `json:"transactions"`
			Meta       struct {
				NextPage *int `json:"next_page"`
			} `json:"meta"`
		}
		if err := c.get(ctx, "/v2/transactions", params, &l); err != nil {
			return nil, false, err
		}
		res = append(res, l.Operations...)
		if l.Meta.NextPage == nil || len(l.Operations) == 0 {
			return res, false, nil
		}
	}
	return res, true, nil
}

// --- Classement ---

// NormaliserIBAN retire espaces et casse pour comparer deux IBAN.
func NormaliserIBAN(s string) string { return strings.ToUpper(strings.ReplaceAll(s, " ", "")) }

// Interne dit si l'opération est un virement entre deux comptes de l'organisation.
func Interne(o Operation, ibans map[string]bool) bool {
	return o.Virement != nil && o.Virement.IBAN != "" && ibans[NormaliserIBAN(o.Virement.IBAN)]
}

// CleLibelle est la clé sous laquelle un choix est retenu pour les opérations suivantes.
func CleLibelle(o Operation) string {
	return strings.Join(strings.Fields(strings.ToLower(o.Nom())), " ")
}

// EstURSSAF dit si un débit paie l'URSSAF.
func EstURSSAF(o Operation) bool {
	return o.Sens == "debit" && strings.Contains(strings.ToUpper(o.Libelle+" "+o.Contrepartie+" "+o.Reference), "URSSAF")
}

// DeclarationPrecedente renvoie le code de la déclaration URSSAF de la période qui précède la
// date du paiement : le mois d'avant (ex. urssaf-2026-09) ou le trimestre d'avant (urssaf-2026-t3).
func DeclarationPrecedente(d time.Time, periodicite string) string {
	if periodicite == "trimestrielle" {
		t := (int(d.Month())-1)/3 + 1
		a := d.Year()
		if t--; t == 0 {
			t, a = 4, a-1
		}
		return fmt.Sprintf("urssaf-%d-t%d", a, t)
	}
	p := time.Date(d.Year(), d.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	return fmt.Sprintf("urssaf-%d-%02d", p.Year(), p.Month())
}
