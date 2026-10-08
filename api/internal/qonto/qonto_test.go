package qonto

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVerifierCle(t *testing.T) {
	if err := VerifierCle("mon-entreprise-1234", "0123456789abcdef"); err != nil {
		t.Errorf("clé valide refusée : %v", err)
	}
	for _, c := range [][2]string{{"", "0123456789abcdef"}, {"mon-entreprise", ""}, {"mon entreprise", "0123456789abcdef"}, {"x", "abc"}, {"x", "a:b0123456"}} {
		if err := VerifierCle(c[0], c[1]); err == nil {
			t.Errorf("%q acceptée", c)
		}
	}
}

func TestDeclarationPrecedente(t *testing.T) {
	cas := []struct{ date, per, code string }{
		{"2026-10-05", "mensuelle", "urssaf-2026-09"},
		{"2026-01-03", "mensuelle", "urssaf-2025-12"},
		{"2026-10-30", "trimestrielle", "urssaf-2026-t3"},
		{"2026-02-01", "trimestrielle", "urssaf-2025-t4"},
	}
	for _, c := range cas {
		d, _ := time.Parse("2006-01-02", c.date)
		if got := DeclarationPrecedente(d, c.per); got != c.code {
			t.Errorf("%s %s : %s, attendu %s", c.date, c.per, got, c.code)
		}
	}
}

func TestClassement(t *testing.T) {
	o := Operation{Sens: "debit", Libelle: "URSSAF D ILE DE FRANCE", Contrepartie: "  Urssaf   Île-de-France "}
	if !EstURSSAF(o) {
		t.Error("paiement URSSAF non reconnu")
	}
	if CleLibelle(o) != "urssaf île-de-france" {
		t.Errorf("clé : %q", CleLibelle(o))
	}
	if EstURSSAF(Operation{Sens: "credit", Libelle: "URSSAF remboursement"}) {
		t.Error("un crédit n'est pas un paiement URSSAF")
	}
	ibans := map[string]bool{NormaliserIBAN("FR76 1234 5678"): true}
	vir := Operation{}
	vir.Virement = &struct {
		IBAN string `json:"counterparty_account_number"`
	}{IBAN: "fr7612345678"}
	if !Interne(vir, ibans) {
		t.Error("virement interne non reconnu")
	}
	vir.Virement.IBAN = "FR7600000000"
	if Interne(vir, ibans) || Interne(Operation{}, ibans) {
		t.Error("virement externe pris pour interne")
	}
}

func TestLecture(t *testing.T) {
	var auth, pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/v2/organization":
			fmt.Fprint(w, `{"organization":{"slug":"ma-boite","legal_name":"Ma Boîte","bank_accounts":[
				{"id":"c1","iban":"FR761","status":"active","currency":"EUR","balance_cents":150000,"main":true},
				{"id":"c2","iban":"FR762","status":"closed","currency":"EUR","balance_cents":0},
				{"id":"c3","iban":"DE1","status":"active","currency":"EUR","balance_cents":999,"is_external_account":true}]}}`)
		case "/v2/transactions":
			q := r.URL.Query()
			if q.Get("bank_account_id") != "c1" || q.Get("status[]") != "completed" || q.Get("settled_at_from") == "" {
				http.Error(w, `{"message":"mauvais paramètres"}`, http.StatusBadRequest)
				return
			}
			pages = append(pages, q.Get("current_page"))
			if q.Get("current_page") == "1" {
				fmt.Fprint(w, `{"transactions":[{"id":"t1","amount_cents":12000,"currency":"EUR","side":"credit","label":"STRIPE","status":"completed","settled_at":"2026-10-01T23:30:00Z"}],"meta":{"next_page":2}}`)
				return
			}
			fmt.Fprint(w, `{"transactions":[{"id":"t2","amount_cents":4500,"currency":"EUR","side":"debit","label":"URSSAF","status":"completed","settled_at":"2026-10-05T09:00:00Z"}],"meta":{"next_page":null}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := &Client{Identifiant: "ma-boite-1", Cle: "secret12", URL: srv.URL}
	org, err := c.Organisation(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if org.Libelle() != "Ma Boîte" || len(org.ComptesSuivis()) != 1 || org.ComptesSuivis()[0].Solde != 150000 {
		t.Errorf("organisation : %+v", org)
	}
	ops, tronque, err := c.Operations(context.Background(), "c1", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), nil)
	if err != nil || tronque || len(ops) != 2 || ops[1].Montant != 4500 || ops[1].Sens != "debit" {
		t.Fatalf("opérations : %+v %v %v", ops, tronque, err)
	}
	if fmt.Sprint(pages) != "[1 2]" || auth[0] != "ma-boite-1:secret12" {
		t.Errorf("pages %v, auth %q", pages, auth[0])
	}
}

func TestErreur(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"errors":[{"code":"unauthorized"}]}`)
	}))
	defer srv.Close()
	_, err := (&Client{Identifiant: "x", Cle: "y", URL: srv.URL}).Organisation(context.Background())
	e, ok := err.(*ErreurQonto)
	if !ok || e.Statut != 401 || e.Message != "identifiant ou clé refusés" {
		t.Errorf("erreur : %v", err)
	}
}
