package stripe

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVerifierCle(t *testing.T) {
	if m, err := VerifierCle("rk_live_abc"); err != nil || m != "live" {
		t.Errorf("rk_live : %s %v", m, err)
	}
	if m, err := VerifierCle("rk_test_abc"); err != nil || m != "test" {
		t.Errorf("rk_test : %s %v", m, err)
	}
	for _, c := range []string{"sk_live_abc", "pk_live_abc", "n'importe quoi", ""} {
		if _, err := VerifierCle(c); err == nil {
			t.Errorf("%q acceptée", c)
		}
	}
}

const abonnements = `[
 {"id":"sub_1","status":"active","currency":"eur","items":{"data":[{"quantity":2,"price":{"unit_amount":1500,"currency":"eur","recurring":{"interval":"month","interval_count":1,"usage_type":"licensed"}}}]}},
 {"id":"sub_2","status":"active","currency":"eur","items":{"data":[{"quantity":1,"price":{"unit_amount":12000,"currency":"eur","recurring":{"interval":"year","interval_count":1,"usage_type":"licensed"}}}]}},
 {"id":"sub_3","status":"past_due","currency":"eur","discount":{"coupon":{"percent_off":50},"end":null},"items":{"data":[{"quantity":1,"price":{"unit_amount":2000,"currency":"eur","recurring":{"interval":"month","interval_count":1,"usage_type":"licensed"}}}]}},
 {"id":"sub_4","status":"active","currency":"eur","discount":{"coupon":{"percent_off":100},"end":1000},"items":{"data":[{"quantity":1,"price":{"unit_amount":900,"currency":"eur","recurring":{"interval":"month","interval_count":3,"usage_type":"licensed"}}}]}},
 {"id":"sub_5","status":"trialing","currency":"eur","items":{"data":[{"quantity":1,"price":{"unit_amount":5000,"currency":"eur","recurring":{"interval":"month","interval_count":1}}}]}},
 {"id":"sub_6","status":"canceled","currency":"eur","items":{"data":[{"quantity":1,"price":{"unit_amount":5000,"currency":"eur","recurring":{"interval":"month","interval_count":1}}}]}},
 {"id":"sub_7","status":"active","currency":"usd","discount":{"coupon":{"amount_off":500,"currency":"usd"}},"items":{"data":[{"quantity":1,"price":{"unit_amount":2000,"currency":"usd","recurring":{"interval":"month","interval_count":1}}}]}},
 {"id":"sub_8","status":"active","currency":"eur","items":{"data":[{"quantity":1,"price":{"unit_amount":null,"currency":"eur","recurring":{"interval":"month","interval_count":1,"usage_type":"metered"}}}]}}
]`

func TestCalculerMRR(t *testing.T) {
	var abos []Abonnement
	if err := json.Unmarshal([]byte(abonnements), &abos); err != nil {
		t.Fatal(err)
	}
	m := CalculerMRR(abos, time.Unix(2000, 0))
	// 2 × 15 € + 120 €/an (10 €) + 20 € à −50 % (10 €) + 9 € par trimestre, remise finie (3 €)
	if m.ParDevise["eur"] != 3000+1000+1000+300 {
		t.Errorf("MRR eur = %d", m.ParDevise["eur"])
	}
	if m.ParDevise["usd"] != 1500 {
		t.Errorf("MRR usd = %d", m.ParDevise["usd"])
	}
	if m.Actifs != 6 || m.EnEssai != 1 || m.NonCalcules != 1 {
		t.Errorf("compteurs = %+v", m)
	}
}

func TestParMois(t *testing.T) {
	paris, _ := time.LoadLocation("Europe/Paris")
	oct := time.Date(2026, 10, 1, 0, 30, 0, 0, paris).Unix() // 30 septembre 22 h 30 UTC : octobre à Paris
	mvts := []Mouvement{
		{Amount: 10000, Fee: 165, Currency: "eur", Created: oct, ReportingCategory: "charge"},
		{Amount: -2000, Fee: 0, Currency: "eur", Created: oct, ReportingCategory: "refund"},
		{Amount: -500, Currency: "eur", Created: oct, ReportingCategory: "fee"},
		{Amount: -8000, Currency: "eur", Created: oct, ReportingCategory: "payout"},
		{Amount: 9999, Currency: "usd", Created: oct, ReportingCategory: "charge"},
		{Amount: 4000, Fee: 85, Currency: "eur", Created: oct - 86400, ReportingCategory: "charge"},
	}
	ms := ParMois(mvts, paris)
	if len(ms) != 2 || ms[0].Mois != "2026-09" || ms[1].Mois != "2026-10" {
		t.Fatalf("mois = %+v", ms)
	}
	o := ms[1]
	if o.Encaisse != 10000 || o.Rembourse != 2000 || o.Frais != 665 || o.Mouvements != 3 {
		t.Errorf("octobre = %+v", o)
	}
}

func TestLire(t *testing.T) {
	var pages int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer rk_test_x" || r.Header.Get("Stripe-Version") != Version {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"message":"Invalid API Key provided"}}`))
			return
		}
		switch r.URL.Path {
		case "/v1/account":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"message":"The provided key does not have the required permissions"}}`))
		case "/v1/subscriptions":
			if r.URL.Query().Get("status") != "all" {
				t.Errorf("status = %q", r.URL.Query().Get("status"))
			}
			_, _ = w.Write([]byte(`{"data":` + abonnements + `,"has_more":false}`))
		case "/v1/balance_transactions":
			pages++
			if r.URL.Query().Get("starting_after") == "" {
				_, _ = w.Write([]byte(`{"data":[{"id":"txn_1","amount":1000,"fee":30,"currency":"eur","created":1790000000,"reporting_category":"charge"}],"has_more":true}`))
				return
			}
			if r.URL.Query().Get("starting_after") != "txn_1" {
				t.Errorf("starting_after = %q", r.URL.Query().Get("starting_after"))
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"txn_2","amount":500,"fee":15,"currency":"eur","created":1790000100,"reporting_category":"charge"}],"has_more":false}`))
		}
	}))
	defer srv.Close()
	paris, _ := time.LoadLocation("Europe/Paris")
	l, err := (&Client{Cle: "rk_test_x", URL: srv.URL}).Lire(context.Background(), time.Unix(1790000200, 0), paris)
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 || len(l.Mois) != 1 || l.Mois[0].Encaisse != 1500 || l.Mois[0].Frais != 45 || l.MRR.Actifs != 6 || l.CompteID != "" {
		t.Errorf("lecture = %+v (pages %d)", l, pages)
	}
	_, err = (&Client{Cle: "rk_test_mauvaise", URL: srv.URL}).Lire(context.Background(), time.Now(), paris)
	if err == nil || !strings.Contains(err.Error(), "Invalid API Key") {
		t.Errorf("erreur attendue, obtenu %v", err)
	}
}
