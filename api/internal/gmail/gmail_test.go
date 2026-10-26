package gmail

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
)

const mailTexte = "From: Paul Martin <Paul@Exemple.fr>\r\n" +
	"To: moi@gmail.com\r\n" +
	"Subject: =?UTF-8?Q?Devis_r=C3=A9vis=C3=A9?=\r\n" +
	"Message-ID: <abc@exemple.fr>\r\n" +
	"Date: Mon, 26 Oct 2026 10:00:00 +0100\r\n" +
	"MIME-Version: 1.0\r\n" +
	"Content-Type: multipart/alternative; boundary=x\r\n\r\n" +
	"--x\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nBonjour,\r\n\r\nvoici le devis   révisé.\r\n" +
	"--x\r\nContent-Type: text/html\r\n\r\n<p>Bonjour</p>\r\n--x--\r\n"

const mailHTML = "From: news@boutique.fr\r\nSubject: Promo\r\nMessage-ID: <promo@boutique.fr>\r\n" +
	"Content-Type: text/html; charset=utf-8\r\n\r\n<html><style>p{color:red}</style><p>Jusqu&rsquo;à -50&nbsp;%</p></html>\r\n"

// serveur lance un serveur IMAP en mémoire, en TLS, avec deux mails dans INBOX (le premier lu).
func serveur(t *testing.T) Compte {
	h := httptest.NewTLSServer(nil)
	certs := h.TLS.Certificates
	h.Close()

	mem := imapmemserver.New()
	u := imapmemserver.NewUser("moi@gmail.com", "app-pass")
	if err := u.Create("INBOX", nil); err != nil {
		t.Fatal(err)
	}
	for i, m := range []string{mailTexte, mailHTML} {
		opts := &imap.AppendOptions{Time: time.Now()}
		if i == 0 {
			opts.Flags = []imap.Flag{imap.FlagSeen}
		}
		if _, err := u.Append("INBOX", lecteur{strings.NewReader(m), int64(len(m))}, opts); err != nil {
			t.Fatal(err)
		}
	}
	mem.AddUser(u)
	srv := imapserver.New(&imapserver.Options{
		NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return mem.NewSession(), nil, nil
		},
		Caps:         imap.CapSet{imap.CapIMAP4rev1: {}, imap.CapMove: {}},
		InsecureAuth: true,
	})
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: certs})
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })
	return Compte{Email: "moi@gmail.com", MotDePasse: "app-pass", Adresse: ln.Addr().String(), TLS: &tls.Config{InsecureSkipVerify: true}}
}

type lecteur struct {
	*strings.Reader
	n int64
}

func (l lecteur) Size() int64 { return l.n }

func TestLire(t *testing.T) {
	c := serveur(t)
	ctx, annuler := context.WithTimeout(context.Background(), 10*time.Second)
	defer annuler()
	msgs, err := c.Lire(ctx, time.Now().AddDate(0, 0, -14), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("%d mails, attendu 2", len(msgs))
	}
	m := msgs[0]
	if m.De != "paul@exemple.fr" || m.DeNom != "Paul Martin" || m.Sujet != "Devis révisé" || m.MessageID != "abc@exemple.fr" || !m.Lu {
		t.Errorf("premier mail : %+v", m)
	}
	if m.Extrait != "Bonjour, voici le devis révisé." {
		t.Errorf("extrait du texte brut : %q", m.Extrait)
	}
	if msgs[1].Extrait != "Jusqu’à -50 %" || msgs[1].Lu {
		t.Errorf("mail HTML : %+v", msgs[1])
	}
}

func TestIdentifiantsRefuses(t *testing.T) {
	c := serveur(t)
	c.MotDePasse = "faux"
	err := c.Verifier(context.Background())
	if !errors.Is(err, ErrIdentifiants) {
		t.Fatalf("Verifier avec un faux mot de passe = %v", err)
	}
}

func TestDelai(t *testing.T) {
	// Un serveur qui accepte la connexion mais ne répond jamais.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			defer c.Close()
		}
	}()
	ctx, annuler := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer annuler()
	debut := time.Now()
	err = Compte{Adresse: ln.Addr().String(), TLS: &tls.Config{InsecureSkipVerify: true}}.Verifier(ctx)
	if err == nil || time.Since(debut) > 5*time.Second {
		t.Fatalf("Verifier sur un serveur muet = %v après %s", err, time.Since(debut))
	}
}

func TestExtraitVide(t *testing.T) {
	if Extrait(nil) != "" || Extrait([]byte("pas un mail")) != "" {
		t.Error("un contenu illisible doit donner un extrait vide")
	}
}
