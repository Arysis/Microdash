package mails

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/arysis/microdash/api/internal/chiffre"
	"github.com/arysis/microdash/api/internal/gmail"
	"github.com/arysis/microdash/api/internal/store"
)

type fausses struct {
	connexion  *store.ConnexionMail
	mails      []store.Mail
	regles     []store.RegleMail
	erreur     string
	supprimees []string
}

func (f *fausses) ConnexionMail(context.Context, int64) (*store.ConnexionMail, error) {
	return f.connexion, nil
}
func (f *fausses) ComptesMail(context.Context) ([]int64, error)                 { return []int64{1}, nil }
func (f *fausses) Mails(context.Context, int64) ([]store.Mail, error)           { return f.mails, nil }
func (f *fausses) ReglesMail(context.Context, int64) ([]store.RegleMail, error) { return f.regles, nil }
func (f *fausses) Mail(_ context.Context, _ int64, ref string) (*store.Mail, error) {
	for _, m := range f.mails {
		if m.Ref == ref {
			return &m, nil
		}
	}
	return nil, store.ErrNotFound
}
func (f *fausses) SupprimerMail(_ context.Context, _ int64, ref string) error {
	f.supprimees = append(f.supprimees, ref)
	return nil
}
func (f *fausses) RemplacerMails(_ context.Context, _ int64, ms []store.Mail, _ time.Time) error {
	f.mails, f.erreur = ms, ""
	return nil
}
func (f *fausses) NoterErreurMail(_ context.Context, _ int64, msg string) error {
	f.erreur = msg
	return nil
}

type fausseBoite struct {
	msgs     []gmail.Message
	err      error
	compte   gmail.Compte
	archives []string
}

func (b *fausseBoite) Lire(_ context.Context, c gmail.Compte, _ time.Time, _ bool) ([]gmail.Message, error) {
	b.compte = c
	return b.msgs, b.err
}
func (b *fausseBoite) Archiver(_ context.Context, _ gmail.Compte, id string) error {
	b.archives = append(b.archives, id)
	return nil
}

func module(t *testing.T, f *fausses, b *fausseBoite) Module {
	cle, err := chiffre.Depuis(strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	mdp, err := cle.Chiffrer([]byte("app-pass"), ContexteCle(1))
	if err != nil {
		t.Fatal(err)
	}
	f.connexion = &store.ConnexionMail{Email: "moi@gmail.com", MotDePasse: mdp, ImportantGmail: true}
	paris, _ := time.LoadLocation("Europe/Paris")
	return Module{Donnees: f, Cle: cle, Boite: b, Paris: paris}
}

func TestImportant(t *testing.T) {
	regles := []store.RegleMail{{Motif: "@educari.fr"}, {Motif: "Paul@Exemple.fr", Libelle: "Client"}}
	cas := []struct {
		de        string
		gmail     bool
		avecGmail bool
		attendu   bool
		raison    string
	}{
		{"prof@educari.fr", false, true, true, "Règle : @educari.fr"},
		{"paul@exemple.fr", false, true, true, "Client"},
		{"news@boutique.fr", true, true, true, "Important pour Gmail"},
		{"news@boutique.fr", true, false, false, ""},
		{"educari.fr@autre.fr", false, true, false, ""},
	}
	for _, c := range cas {
		imp, raison := Important(store.Mail{De: c.de, ImportantGmail: c.gmail}, regles, c.avecGmail)
		if imp != c.attendu || raison != c.raison {
			t.Errorf("%s (gmail %v, %v) = %v %q", c.de, c.gmail, c.avecGmail, imp, raison)
		}
	}
}

func TestSynchroEtATraiter(t *testing.T) {
	f := &fausses{}
	b := &fausseBoite{msgs: []gmail.Message{
		{MessageID: "a@x", De: "paul@exemple.fr", DeNom: "Paul", Sujet: "Devis", Extrait: "Bonjour", Recu: time.Date(2026, 10, 25, 23, 30, 0, 0, time.UTC), Important: true},
		{MessageID: "b@x", De: "news@boutique.fr", Sujet: "Promo", Recu: time.Now()},
	}}
	m := module(t, f, b)
	if err := m.Synchroniser(context.Background(), 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	if b.compte.MotDePasse != "app-pass" || len(f.mails) != 2 {
		t.Fatalf("synchro : compte %+v, %d mails", b.compte, len(f.mails))
	}
	els, err := m.ATraiter(context.Background(), 1, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(els) != 1 {
		t.Fatalf("%d éléments, attendu 1 : %+v", len(els), els)
	}
	e := els[0]
	if e.Ref != Ref("a@x") || e.Titre != "Devis" || e.Detail != "Paul : Bonjour (Important pour Gmail)" || e.Date != "2026-10-26" || !e.Finir || e.Limite {
		t.Errorf("élément : %+v", e)
	}
	if !strings.HasPrefix(e.Lien, "https://mail.google.com/mail/u/0/#search/rfc822msgid:a@x") {
		t.Errorf("lien : %s", e.Lien)
	}

	if err := m.Finir(context.Background(), 1, e.Ref); err != nil {
		t.Fatal(err)
	}
	if len(b.archives) != 1 || b.archives[0] != "a@x" || len(f.supprimees) != 1 {
		t.Errorf("archivage : %v, supprimés %v", b.archives, f.supprimees)
	}
}

func TestSynchroRefusee(t *testing.T) {
	f := &fausses{}
	m := module(t, f, &fausseBoite{err: gmail.ErrIdentifiants})
	if err := m.Synchroniser(context.Background(), 1, time.Now()); !errors.Is(err, gmail.ErrIdentifiants) {
		t.Fatalf("erreur = %v", err)
	}
	if !strings.Contains(f.erreur, "mot de passe d'application") {
		t.Errorf("erreur notée : %q", f.erreur)
	}
	f.connexion.Erreur = f.erreur
	els, _ := m.ATraiter(context.Background(), 1, time.Now())
	if len(els) != 1 || els[0].Ref != refErreur || els[0].Finir {
		t.Errorf("rappel d'erreur : %+v", els)
	}
}
