// Package mails est le module Mails du copilote : il copie régulièrement l'enveloppe des mails
// récents de la boîte de réception Gmail, et met les mails importants dans « Ce soir ».
// « Fait » archive le mail dans Gmail.
package mails

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/arysis/microdash/api/internal/chiffre"
	"github.com/arysis/microdash/api/internal/gmail"
	"github.com/arysis/microdash/api/internal/modules"
	"github.com/arysis/microdash/api/internal/store"
)

const (
	codeSource = "mails"
	// Jours de mails gardés en copie.
	Jours = 14
	// Intervalle entre deux synchros automatiques.
	Intervalle = 5 * time.Minute
	refErreur  = "connexion"
)

// Donnees est ce que le module lit et écrit en base.
type Donnees interface {
	ConnexionMail(ctx context.Context, userID int64) (*store.ConnexionMail, error)
	ComptesMail(ctx context.Context) ([]int64, error)
	Mails(ctx context.Context, userID int64) ([]store.Mail, error)
	Mail(ctx context.Context, userID int64, ref string) (*store.Mail, error)
	SupprimerMail(ctx context.Context, userID int64, ref string) error
	RemplacerMails(ctx context.Context, userID int64, mails []store.Mail, le time.Time) error
	NoterErreurMail(ctx context.Context, userID int64, msg string) error
	ReglesMail(ctx context.Context, userID int64) ([]store.RegleMail, error)
}

// Boite lit et archive les mails ; gmail.Compte en est la version réelle.
type Boite interface {
	Lire(ctx context.Context, c gmail.Compte, depuis time.Time, avecImportant bool) ([]gmail.Message, error)
	Archiver(ctx context.Context, c gmail.Compte, messageID string) error
}

// Gmail est la boîte réelle, en IMAP.
type Gmail struct{}

func (Gmail) Lire(ctx context.Context, c gmail.Compte, depuis time.Time, avecImportant bool) ([]gmail.Message, error) {
	return c.Lire(ctx, depuis, avecImportant)
}
func (Gmail) Archiver(ctx context.Context, c gmail.Compte, messageID string) error {
	return c.Archiver(ctx, messageID)
}

type Module struct {
	Donnees Donnees
	Cle     *chiffre.Cle // nil : connexion impossible (CLE_CHIFFREMENT manquante)
	Boite   Boite
	Paris   *time.Location
}

func (Module) Code() string { return codeSource }
func (Module) Nom() string  { return "Mails" }

// ContexteCle lie le mot de passe chiffré à son compte.
func ContexteCle(uid int64) string { return "gmail:" + strconv.FormatInt(uid, 10) }

// Ref identifie un mail par son Message-ID, sans le montrer.
func Ref(messageID string) string {
	h := sha256.Sum256([]byte(messageID))
	return "m-" + hex.EncodeToString(h[:10])
}

// LienGmail ouvre le mail dans Gmail (recherche par Message-ID).
func LienGmail(messageID string) string {
	return "https://mail.google.com/mail/u/0/#search/" + url.PathEscape("rfc822msgid:"+messageID)
}

// Important dit si un mail compte pour la personne, et pourquoi. Une règle passe avant
// le tri de Gmail.
func Important(m store.Mail, regles []store.RegleMail, avecGmail bool) (bool, string) {
	for _, r := range regles {
		motif := strings.ToLower(r.Motif)
		if m.De == motif || (strings.HasPrefix(motif, "@") && strings.HasSuffix(m.De, motif)) {
			if r.Libelle != "" {
				return true, r.Libelle
			}
			return true, "Règle : " + r.Motif
		}
	}
	if avecGmail && m.ImportantGmail {
		return true, "Important pour Gmail"
	}
	return false, ""
}

func (m Module) compte(c *store.ConnexionMail, uid int64) (gmail.Compte, error) {
	if m.Cle == nil {
		return gmail.Compte{}, errors.New("connexion Gmail désactivée sur ce serveur (CLE_CHIFFREMENT manquante)")
	}
	mdp, err := m.Cle.Dechiffrer(c.MotDePasse, ContexteCle(uid))
	if err != nil {
		return gmail.Compte{}, err
	}
	return gmail.Compte{Email: c.Email, MotDePasse: string(mdp)}, nil
}

// ATraiter renvoie les mails importants encore dans la boîte de réception, et un rappel si
// la synchro échoue.
func (m Module) ATraiter(ctx context.Context, uid int64, _ time.Time) ([]modules.Element, error) {
	c, err := m.Donnees.ConnexionMail(ctx, uid)
	if err != nil || c == nil {
		return nil, err
	}
	var res []modules.Element
	if c.Erreur != "" {
		res = append(res, modules.Element{
			Source: codeSource, Ref: refErreur, Titre: "Gmail ne répond plus",
			Detail: c.Erreur, Importance: 1, Lien: "/reglages",
		})
	}
	ms, err := m.Donnees.Mails(ctx, uid)
	if err != nil {
		return nil, err
	}
	regles, err := m.Donnees.ReglesMail(ctx, uid)
	if err != nil {
		return nil, err
	}
	for _, mail := range ms {
		imp, raison := Important(mail, regles, c.ImportantGmail)
		if !imp {
			continue
		}
		res = append(res, modules.Element{
			Source: codeSource, Ref: mail.Ref, Titre: Sujet(mail), Detail: Detail(mail, raison),
			Date: mail.Recu.In(m.Paris).Format("2006-01-02"), Lien: LienGmail(mail.MessageID), Finir: true,
		})
	}
	return res, nil
}

// Sujet renvoie le sujet du mail, ou « (sans objet) ».
func Sujet(m store.Mail) string {
	if strings.TrimSpace(m.Sujet) == "" {
		return "(sans objet)"
	}
	return m.Sujet
}

// Detail : « Paul Martin : Bonjour, voici le devis… (Important pour Gmail) ».
func Detail(m store.Mail, raison string) string {
	de := m.DeNom
	if de == "" {
		de = m.De
	}
	d := de
	if m.Extrait != "" {
		d += " : " + m.Extrait
	}
	if raison != "" {
		d += " (" + raison + ")"
	}
	return d
}

// Finir archive le mail dans Gmail et le retire de la copie.
func (m Module) Finir(ctx context.Context, uid int64, ref string) error {
	if ref == refErreur {
		return nil
	}
	mail, err := m.Donnees.Mail(ctx, uid, ref)
	if errors.Is(err, store.ErrNotFound) {
		return nil // déjà archivé ou sorti de la copie
	}
	if err != nil {
		return err
	}
	if err := m.Archiver(ctx, uid, mail.MessageID); err != nil {
		return err
	}
	return m.Donnees.SupprimerMail(ctx, uid, ref)
}

// Archiver sort un mail de la boîte de réception Gmail.
func (m Module) Archiver(ctx context.Context, uid int64, messageID string) error {
	c, err := m.Donnees.ConnexionMail(ctx, uid)
	if err != nil {
		return err
	}
	if c == nil {
		return errors.New("Gmail n'est pas connecté")
	}
	cpt, err := m.compte(c, uid)
	if err != nil {
		return err
	}
	ctx, annuler := context.WithTimeout(ctx, 12*time.Second)
	defer annuler()
	return m.Boite.Archiver(ctx, cpt, messageID)
}

// Synchroniser recopie les mails des 14 derniers jours. Une erreur est aussi notée sur la
// connexion, pour l'afficher.
func (m Module) Synchroniser(ctx context.Context, uid int64, maintenant time.Time) error {
	c, err := m.Donnees.ConnexionMail(ctx, uid)
	if err != nil || c == nil {
		return err
	}
	cpt, err := m.compte(c, uid)
	if err == nil {
		var msgs []gmail.Message
		lctx, annuler := context.WithTimeout(ctx, 45*time.Second)
		msgs, err = m.Boite.Lire(lctx, cpt, maintenant.AddDate(0, 0, -Jours), c.ImportantGmail)
		annuler()
		if err == nil {
			copie := make([]store.Mail, 0, len(msgs))
			for _, g := range msgs {
				copie = append(copie, store.Mail{
					Ref: Ref(g.MessageID), MessageID: g.MessageID, De: g.De, DeNom: g.DeNom,
					Sujet: g.Sujet, Extrait: g.Extrait, Recu: g.Recu, Lu: g.Lu, ImportantGmail: g.Important,
				})
			}
			return m.Donnees.RemplacerMails(ctx, uid, copie, maintenant)
		}
	}
	msg := err.Error()
	if errors.Is(err, gmail.ErrIdentifiants) {
		msg = "Gmail refuse le mot de passe d'application : crée-en un nouveau et reconnecte Gmail dans Réglages."
	}
	if e := m.Donnees.NoterErreurMail(ctx, uid, msg); e != nil {
		return e
	}
	return fmt.Errorf("synchro Gmail : %w", err)
}

// Planifier synchronise tous les comptes connectés, toutes les 5 minutes.
func (m Module) Planifier(ctx context.Context) {
	for {
		ids, err := m.Donnees.ComptesMail(ctx)
		if err != nil {
			slog.Error("synchro Gmail : comptes", "err", err)
		}
		for _, uid := range ids {
			if err := m.Synchroniser(ctx, uid, time.Now()); err != nil {
				slog.Warn("synchro Gmail", "compte", uid, "err", err)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(Intervalle):
		}
	}
}
