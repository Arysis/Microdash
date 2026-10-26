// Package gmail lit la boîte de réception Gmail en IMAP, avec un mot de passe d'application :
// enveloppe et début du texte des mails récents, sans garder les messages complets.
package gmail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-message/charset"
	"github.com/emersion/go-message/mail"
)

// Adresse du serveur IMAP de Gmail.
const Adresse = "imap.gmail.com:993"

// Octets lus au début de chaque mail pour l'extrait.
const debutLu = 16 * 1024

// ErrIdentifiants : Gmail refuse l'adresse ou le mot de passe d'application.
var ErrIdentifiants = errors.New("Gmail refuse l'adresse ou le mot de passe d'application")

// Message est un mail de la boîte de réception.
type Message struct {
	MessageID string
	De        string // adresse, en minuscules
	DeNom     string
	Sujet     string
	Extrait   string
	Recu      time.Time
	Lu        bool
	Important bool // dans la boîte « Important » de Gmail
}

type Compte struct {
	Email      string
	MotDePasse string
	Adresse    string      // vide : Gmail ; une autre adresse sert aux tests
	TLS        *tls.Config // tests uniquement
}

// avec ouvre une session IMAP, lance f, et coupe tout si le contexte expire.
func (c Compte) avec(ctx context.Context, f func(*imapclient.Client) error) error {
	adresse := c.Adresse
	if adresse == "" {
		adresse = Adresse
	}
	cfg := c.TLS
	if cfg == nil {
		hote, _, _ := net.SplitHostPort(adresse)
		cfg = &tls.Config{ServerName: hote}
	}
	d := &tls.Dialer{NetDialer: &net.Dialer{Timeout: 15 * time.Second}, Config: cfg}
	conn, err := d.DialContext(ctx, "tcp", adresse)
	if err != nil {
		return fmt.Errorf("connexion à Gmail impossible : %w", err)
	}
	cl := imapclient.New(conn, &imapclient.Options{WordDecoder: &mime.WordDecoder{CharsetReader: charset.Reader}})
	defer cl.Close()
	fini := make(chan error, 1)
	go func() {
		if err := cl.Login(c.Email, c.MotDePasse).Wait(); err != nil {
			var ie *imap.Error
			if errors.As(err, &ie) {
				fini <- ErrIdentifiants
				return
			}
			fini <- err
			return
		}
		err := f(cl)
		_ = cl.Logout().Wait()
		fini <- err
	}()
	select {
	case err := <-fini:
		return err
	case <-ctx.Done():
		cl.Close()
		return fmt.Errorf("Gmail ne répond pas : %w", ctx.Err())
	}
}

// Verifier essaie de se connecter avec l'adresse et le mot de passe d'application.
func (c Compte) Verifier(ctx context.Context) error {
	return c.avec(ctx, func(*imapclient.Client) error { return nil })
}

// boite cherche le nom d'une boîte spéciale de Gmail (\All, \Important…).
func boite(cl *imapclient.Client, attr imap.MailboxAttr) (string, error) {
	boites, err := cl.List("", "*", nil).Collect()
	if err != nil {
		return "", err
	}
	for _, b := range boites {
		for _, a := range b.Attrs {
			if strings.EqualFold(string(a), string(attr)) {
				return b.Mailbox, nil
			}
		}
	}
	return "", nil
}

const attrImportant = imap.MailboxAttr(`\Important`)

// Lire renvoie les mails de la boîte de réception reçus depuis la date donnée. avecImportant :
// noter ceux que Gmail range dans « Important ».
func (c Compte) Lire(ctx context.Context, depuis time.Time, avecImportant bool) ([]Message, error) {
	var res []Message
	err := c.avec(ctx, func(cl *imapclient.Client) error {
		importants := map[string]bool{}
		if avecImportant {
			nom, err := boite(cl, attrImportant)
			if err != nil {
				return err
			}
			if nom != "" {
				if importants, err = identifiants(cl, nom, depuis); err != nil {
					return err
				}
			}
		}
		if _, err := cl.Select("INBOX", &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
			return err
		}
		uids, err := chercher(cl, depuis)
		if err != nil || len(uids) == 0 {
			return err
		}
		section := &imap.FetchItemBodySection{Peek: true, Partial: &imap.SectionPartial{Offset: 0, Size: debutLu}}
		msgs, err := cl.Fetch(imap.UIDSetNum(uids...), &imap.FetchOptions{
			Envelope: true, Flags: true, InternalDate: true, UID: true,
			BodySection: []*imap.FetchItemBodySection{section},
		}).Collect()
		if err != nil {
			return err
		}
		for _, m := range msgs {
			if m.Envelope == nil {
				continue
			}
			msg := Message{
				MessageID: m.Envelope.MessageID, Sujet: m.Envelope.Subject, Recu: m.InternalDate,
				Extrait: Extrait(m.FindBodySection(section)),
			}
			if len(m.Envelope.From) > 0 {
				msg.De = strings.ToLower(m.Envelope.From[0].Addr())
				msg.DeNom = m.Envelope.From[0].Name
			}
			for _, f := range m.Flags {
				if f == imap.FlagSeen {
					msg.Lu = true
				}
			}
			msg.Important = importants[msg.MessageID]
			if msg.MessageID == "" {
				msg.MessageID = fmt.Sprintf("uid-%d@inbox", m.UID)
			}
			res = append(res, msg)
		}
		return nil
	})
	return res, err
}

func chercher(cl *imapclient.Client, depuis time.Time) ([]imap.UID, error) {
	data, err := cl.UIDSearch(&imap.SearchCriteria{Since: depuis}, nil).Wait()
	if err != nil {
		return nil, err
	}
	return data.AllUIDs(), nil
}

// identifiants renvoie les Message-ID des mails d'une boîte reçus depuis la date donnée.
func identifiants(cl *imapclient.Client, nom string, depuis time.Time) (map[string]bool, error) {
	res := map[string]bool{}
	if _, err := cl.Select(nom, &imap.SelectOptions{ReadOnly: true}).Wait(); err != nil {
		return nil, err
	}
	uids, err := chercher(cl, depuis)
	if err != nil || len(uids) == 0 {
		return res, err
	}
	msgs, err := cl.Fetch(imap.UIDSetNum(uids...), &imap.FetchOptions{Envelope: true}).Collect()
	if err != nil {
		return nil, err
	}
	for _, m := range msgs {
		if m.Envelope != nil && m.Envelope.MessageID != "" {
			res[m.Envelope.MessageID] = true
		}
	}
	return res, nil
}

// Archiver sort un mail de la boîte de réception (il reste dans « Tous les messages »).
// Un mail déjà archivé ou supprimé n'est pas une erreur.
func (c Compte) Archiver(ctx context.Context, messageID string) error {
	return c.avec(ctx, func(cl *imapclient.Client) error {
		tous, err := boite(cl, imap.MailboxAttrAll)
		if err != nil {
			return err
		}
		if tous == "" {
			return errors.New("boîte « Tous les messages » introuvable : l'accès IMAP à tous les libellés est-il coupé dans Gmail ?")
		}
		if _, err := cl.Select("INBOX", nil).Wait(); err != nil {
			return err
		}
		data, err := cl.UIDSearch(&imap.SearchCriteria{
			Header: []imap.SearchCriteriaHeaderField{{Key: "Message-ID", Value: messageID}},
		}, nil).Wait()
		if err != nil {
			return err
		}
		uids := data.AllUIDs()
		if len(uids) == 0 {
			return nil
		}
		_, err = cl.Move(imap.UIDSetNum(uids...), tous).Wait()
		return err
	})
}

var (
	balises = regexp.MustCompile(`(?s)<(style|script)[^>]*>.*?</(style|script)>|<[^>]+>`)
	blancs  = regexp.MustCompile(`\s+`)
	entites = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&#39;", "'", "&rsquo;", "’", "&eacute;", "é", "&egrave;", "è", "&agrave;", "à")
)

// Extrait renvoie les 200 premiers caractères du texte d'un mail, à partir de son début brut
// (en-têtes compris, éventuellement tronqué). Le texte brut est préféré au HTML.
func Extrait(brut []byte) string {
	if len(brut) == 0 {
		return ""
	}
	r, err := mail.CreateReader(strings.NewReader(string(brut)))
	if err != nil && r == nil {
		return ""
	}
	var html string
	for {
		p, err := r.NextPart()
		if err != nil {
			break
		}
		h, ok := p.Header.(*mail.InlineHeader)
		if !ok {
			continue
		}
		type_, _, _ := h.ContentType()
		b, _ := io.ReadAll(io.LimitReader(p.Body, 4096))
		switch {
		case type_ == "text/plain" && len(strings.TrimSpace(string(b))) > 0:
			return couper(string(b))
		case type_ == "text/html" && html == "":
			html = entites.Replace(balises.ReplaceAllString(string(b), " "))
		}
	}
	return couper(html)
}

func couper(s string) string {
	s = strings.TrimSpace(blancs.ReplaceAllString(strings.ToValidUTF8(s, ""), " "))
	if utf8.RuneCountInString(s) <= 200 {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:199])) + "…"
}
