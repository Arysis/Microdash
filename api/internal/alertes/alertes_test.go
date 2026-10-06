package alertes

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/arysis/microdash/api/baremes"
	"github.com/arysis/microdash/api/internal/bareme"
	"github.com/arysis/microdash/api/internal/calc"
	"github.com/arysis/microdash/api/internal/store"
)

func jour(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestSignature(t *testing.T) {
	s := Signature("un-secret-de-test-assez-long-pour-hmac")
	j := s.Jeton(42)
	if id, ok := s.Verifier(j); !ok || id != 42 {
		t.Fatalf("Verifier(%q) = %d, %v", j, id, ok)
	}
	for _, faux := range []string{"", "42", "43." + strings.SplitN(j, ".", 2)[1], j + "x"} {
		if _, ok := s.Verifier(faux); ok {
			t.Errorf("jeton %q accepté", faux)
		}
	}
	if _, ok := Signature("autre-secret").Verifier(j); ok {
		t.Error("jeton accepté avec un autre secret")
	}
}

func TestProchainPassage(t *testing.T) {
	cas := map[string]string{
		"2026-10-06T05:59:00Z": "2026-10-06T08:00:00+02:00", // 7 h 59 à Paris
		"2026-10-06T06:00:00Z": "2026-10-07T08:00:00+02:00", // 8 h pile : demain
		"2026-10-24T22:30:00Z": "2026-10-25T08:00:00+01:00", // passage à l'heure d'hiver
	}
	for depuis, attendu := range cas {
		d, _ := time.Parse(time.RFC3339, depuis)
		if got := ProchainPassage(d).Format(time.RFC3339); got != attendu {
			t.Errorf("ProchainPassage(%s) = %s, attendu %s", depuis, got, attendu)
		}
	}
	// 23 h 30 à Paris le 5 octobre : on est déjà le 5 à Paris, pas le 6.
	if got := Aujourdhui(time.Date(2026, 10, 5, 21, 30, 0, 0, time.UTC)); !got.Equal(jour("2026-10-05")) {
		t.Errorf("Aujourdhui = %s", got)
	}
}

var tous = store.Preferences{Echeances: true, Plafond: true, TVA: true, CFE: true}

func TestRappels(t *testing.T) {
	agenda := []calc.Echeance{
		{Code: "urssaf-2026-t3", Type: "urssaf", Libelle: "Déclaration URSSAF du 3e trimestre 2026", Date: "2026-11-02", BaremeExact: true, Note: "Aucune recette sur la période : la déclaration est due quand même, à 0 €."},
		{Code: "revenus-2025", Type: "revenus", Libelle: "Déclaration de revenus 2025"}, // sans date
		{Code: "cfe-2026", Type: "cfe", Libelle: "CFE 2026", Date: "2026-12-15", Note: "À confirmer pour ton cas."},
	}
	d := store.Destinataire{ID: 1, Preferences: tous}
	cas := []struct {
		jour   string
		faites map[string]time.Time
		prefs  store.Preferences
		cles   []string
	}{
		{"2026-10-25", nil, tous, nil},                                        // 8 jours avant : rien
		{"2026-10-26", nil, tous, []string{"urssaf-2026-t3:j7"}},              // 7 jours avant
		{"2026-10-31", nil, tous, []string{"urssaf-2026-t3:j7"}},              // 2 jours avant (passage manqué)
		{"2026-11-01", nil, tous, []string{"urssaf-2026-t3:j1"}},              // la veille
		{"2026-11-02", nil, tous, []string{"urssaf-2026-t3:j1"}},              // le jour même
		{"2026-11-03", nil, tous, nil},                                        // passée
		{"2026-11-01", map[string]time.Time{"urssaf-2026-t3": {}}, tous, nil}, // cochée
		{"2026-11-01", nil, store.Preferences{CFE: true}, nil},                // rappels d'échéances coupés
		{"2026-12-08", nil, tous, []string{"cfe-2026:j7"}},
		{"2026-12-08", nil, store.Preferences{Echeances: true}, nil}, // CFE coupée
	}
	for _, c := range cas {
		d.Preferences = c.prefs
		var cles []string
		for _, a := range Rappels(d, agenda, c.faites, jour(c.jour)) {
			cles = append(cles, a.Cle)
		}
		if strings.Join(cles, ",") != strings.Join(c.cles, ",") {
			t.Errorf("%s (%+v) : %v, attendu %v", c.jour, c.prefs, cles, c.cles)
		}
	}
	d.Preferences = tous
	a := Rappels(d, agenda, nil, jour("2026-10-26"))[0]
	for _, morceau := range []string{"lundi 2 novembre 2026", "dans 7 jours", "0,00 €", "due quand même", "https://www.autoentrepreneur.urssaf.fr"} {
		if !strings.Contains(a.Corps, morceau) {
			t.Errorf("corps sans %q :\n%s", morceau, a.Corps)
		}
	}
	if a.Sujet != "Déclaration URSSAF du 3e trimestre 2026 : dans 7 jours" {
		t.Errorf("sujet = %q", a.Sujet)
	}
}

func TestPlafonds(t *testing.T) {
	r := &calc.Resultat{Annee: 2026, Plafonds: []calc.Plafond{
		{Code: "micro_services", CA: 7000000, Plafond: 8360000, Ratio: 0.837, Niveau: "attention"},
		{Code: "tva_services", CA: 3800000, Plafond: 3750000, Majore: 4125000, Ratio: 1.013, Niveau: "depasse"},
	}}
	d := store.Destinataire{ID: 1, Preferences: tous}
	as := Plafonds(d, r)
	if len(as) != 2 || as[0].Cle != "plafond-2026-micro_services:80" || as[1].Cle != "plafond-2026-tva_services:100" {
		t.Fatalf("alertes = %+v", as)
	}
	if strings.Join(as[1].Aussi, ",") != "plafond-2026-tva_services:80" || !strings.Contains(as[1].Corps, "1er janvier suivant") {
		t.Errorf("TVA dépassée = %+v", as[1])
	}
	if !strings.Contains(as[0].Corps, "83 %") {
		t.Errorf("pourcentage absent :\n%s", as[0].Corps)
	}

	r.Plafonds[1].CA = 4200000 // au-delà du seuil majoré
	as = Plafonds(d, r)
	if as[1].Cle != "plafond-2026-tva_services:majore" || !strings.Contains(as[1].Corps, "dès le jour du dépassement") {
		t.Errorf("TVA majorée = %+v", as[1])
	}

	d.Preferences = store.Preferences{Plafond: true}
	if as := Plafonds(d, r); len(as) != 1 || !strings.HasPrefix(as[0].Cle, "plafond-2026-micro") {
		t.Errorf("TVA coupée : %+v", as)
	}
}

// --- Tourner, avec une fausse base et un faux Resend ---

type fausseSource struct {
	dests   []store.Destinataire
	txs     []store.Transaction
	faites  map[string]time.Time
	envoyes map[string]bool
}

func (f *fausseSource) Transactions(_ context.Context, _ int64, du, au time.Time) ([]store.Transaction, error) {
	var res []store.Transaction
	for _, t := range f.txs {
		if !t.Date.Before(du) && !t.Date.After(au) {
			res = append(res, t)
		}
	}
	return res, nil
}
func (f *fausseSource) EcheancesFaites(context.Context, int64) (map[string]time.Time, error) {
	return f.faites, nil
}
func (f *fausseSource) Destinataires(context.Context) ([]store.Destinataire, error) {
	return f.dests, nil
}
func (f *fausseSource) ReserverAlerte(_ context.Context, uid int64, cle string) (bool, error) {
	k := string(rune('0'+uid)) + cle
	if f.envoyes[k] {
		return false, nil
	}
	f.envoyes[k] = true
	return true, nil
}
func (f *fausseSource) AnnulerAlerte(_ context.Context, uid int64, cle string) error {
	delete(f.envoyes, string(rune('0'+uid))+cle)
	return nil
}

type fauxEnvoyeur struct {
	messages []Message
	panne    bool
}

func (f *fauxEnvoyeur) Envoyer(_ context.Context, m Message) error {
	if f.panne {
		return errors.New("panne")
	}
	f.messages = append(f.messages, m)
	return nil
}

func charger(t *testing.T) *bareme.Set {
	t.Helper()
	set, err := bareme.Load(baremes.FS)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func TestTourner(t *testing.T) {
	profil := store.Profil{Categorie: "services_bic", NatureCFP: "artisan", DebutActivite: jour("2024-03-01"), Periodicite: "trimestrielle"}
	src := &fausseSource{
		dests: []store.Destinataire{
			{ID: 1, Email: "moi@exemple.fr", Profil: profil, Preferences: tous},
			{ID: 2, Email: "autre@exemple.fr", Profil: profil, Preferences: tous},
		},
		txs:     []store.Transaction{{Type: "recette", Date: jour("2026-08-10"), Centimes: 7000000, Categorie: "services_bic"}},
		envoyes: map[string]bool{},
	}
	env := &fauxEnvoyeur{}
	svc := &Service{Source: src, Baremes: charger(t), Envoyeur: env, Signature: Signature("secret-de-test-assez-long-pour-hmac!"), AppURL: "https://app.exemple.fr", LimiterA: []string{" MOI@exemple.fr"}}
	matin := time.Date(2026, 10, 26, 7, 0, 0, 0, time.UTC) // 8 h à Paris, 7 jours avant le 2 novembre

	if err := svc.Tourner(context.Background(), matin); err != nil {
		t.Fatal(err)
	}
	// Échéance du 2 novembre, plafond micro à 80 % et TVA dépassée ; « autre » n'est pas autorisé.
	if len(env.messages) != 3 {
		t.Fatalf("%d e-mails : %+v", len(env.messages), env.messages)
	}
	for _, m := range env.messages {
		if m.A != "moi@exemple.fr" {
			t.Errorf("e-mail envoyé à %s", m.A)
		}
		if !strings.Contains(m.Texte, "https://app.exemple.fr/desinscription?jeton=1.") || !strings.HasPrefix(m.Desinscription, "https://app.exemple.fr/api/alertes/desinscription?jeton=1.") {
			t.Errorf("liens de désinscription absents : %+v", m)
		}
	}
	if !src.envoyes["1plafond-2026-tva_services:80"] {
		t.Error("le seuil 80 % de TVA doit être marqué quand 100 % part directement")
	}

	// Deuxième passage le même jour : rien ne repart.
	if err := svc.Tourner(context.Background(), matin.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(env.messages) != 3 {
		t.Errorf("doublons : %d e-mails", len(env.messages))
	}

	// Un envoi en panne est retenté au passage suivant.
	veille := time.Date(2026, 11, 1, 7, 0, 0, 0, time.UTC)
	env.panne = true
	if err := svc.Tourner(context.Background(), veille); err != nil {
		t.Fatal(err)
	}
	env.panne = false
	if err := svc.Tourner(context.Background(), veille.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(env.messages) != 4 || !strings.Contains(env.messages[3].Sujet, "demain") {
		t.Errorf("rappel de la veille après panne : %+v", env.messages[3:])
	}
}

func TestResend(t *testing.T) {
	var recu map[string]any
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		if r.URL.Path != "/emails" {
			http.NotFound(w, r)
			return
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &recu)
		if recu["to"].([]any)[0] == "refus@exemple.fr" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"domaine non vérifié"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))
	defer srv.Close()
	r := &Resend{Cle: "re_test", De: "Microdash <rappels@exemple.fr>", URL: srv.URL}
	m := Message{A: "moi@exemple.fr", Sujet: "Sujet", Texte: "Texte", Desinscription: "https://app/api/alertes/desinscription?jeton=x"}
	if err := r.Envoyer(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	h := recu["headers"].(map[string]any)
	if auth != "Bearer re_test" || recu["from"] != "Microdash <rappels@exemple.fr>" || recu["text"] != "Texte" ||
		h["List-Unsubscribe"] != "<"+m.Desinscription+">" || h["List-Unsubscribe-Post"] != "List-Unsubscribe=One-Click" {
		t.Errorf("requête = %v (auth %q)", recu, auth)
	}
	m.A = "refus@exemple.fr"
	if err := r.Envoyer(context.Background(), m); err == nil || !strings.Contains(err.Error(), "domaine non vérifié") {
		t.Errorf("erreur attendue, obtenu %v", err)
	}
}
