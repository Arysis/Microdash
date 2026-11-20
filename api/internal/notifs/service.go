package notifs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/arysis/microdash/api/internal/modules"
	"github.com/arysis/microdash/api/internal/store"
	"github.com/arysis/microdash/api/internal/webpush"
)

// Donnees est ce que le service lit et écrit en base.
type Donnees interface {
	Abonnements(ctx context.Context, userID int64) ([]store.AbonnementPush, error)
	PreferencesNotifications(ctx context.Context, userID int64) (store.PreferencesNotifications, error)
	ReserverNotification(ctx context.Context, userID int64, cle string) (bool, error)
	AnnulerNotification(ctx context.Context, userID int64, cle string) error
	OublierAbonnement(ctx context.Context, id int64) error
	NoterErreurAbonnement(ctx context.Context, id int64, msg string) error
	PurgerNotifications(ctx context.Context) error
}

// Pousseur envoie un message chiffré à un appareil (webpush.Envoyeur).
type Pousseur interface {
	Envoyer(ctx context.Context, ab webpush.Abonnement, message []byte, o webpush.Options) error
}

// Agenda donne les choses datées des modules actifs d'un compte, entre deux jours inclus.
type Agenda func(ctx context.Context, userID int64, du, au time.Time) ([]modules.Evenement, error)

// Service envoie les notifications de tous les comptes abonnés.
type Service struct {
	Donnees     Donnees
	Push        Pousseur
	Agenda      Agenda
	ClePublique string // clé VAPID publique, donnée aux navigateurs
}

// Pousser envoie une notification à tous les appareils d'un compte et renvoie le nombre
// d'appareils atteints. Un appareil que son service ne connaît plus est oublié.
func (s *Service) Pousser(ctx context.Context, uid int64, n Notification) (int, error) {
	abs, err := s.Donnees.Abonnements(ctx, uid)
	if err != nil {
		return 0, err
	}
	message, err := json.Marshal(n)
	if err != nil {
		return 0, err
	}
	o := webpush.Options{TTL: n.TTL, Urgence: n.Urgence}
	atteints := 0
	var derniere error
	for _, a := range abs {
		err := s.Push.Envoyer(ctx, webpush.Abonnement{Endpoint: a.Endpoint, P256dh: a.P256dh, Auth: a.Auth}, message, o)
		switch {
		case errors.Is(err, webpush.ErrExpire):
			slog.Info("notifications : appareil oublié", "compte", uid, "appareil", a.Appareil)
			_ = s.Donnees.OublierAbonnement(ctx, a.ID)
		case err != nil:
			slog.Warn("notifications : envoi", "compte", uid, "appareil", a.Appareil, "err", err)
			_ = s.Donnees.NoterErreurAbonnement(ctx, a.ID, err.Error())
			derniere = err
		default:
			atteints++
			if a.Erreur != "" {
				_ = s.Donnees.NoterErreurAbonnement(ctx, a.ID, "")
			}
		}
	}
	if atteints == 0 && derniere != nil {
		return 0, derniere
	}
	return atteints, nil
}

// Tourner envoie les notifications dues maintenant, pour chaque compte qui a un appareil abonné.
func (s *Service) Tourner(ctx context.Context, maintenant time.Time) error {
	abs, err := s.Donnees.Abonnements(ctx, 0)
	if err != nil {
		return err
	}
	vus := map[int64]bool{}
	for _, a := range abs {
		if vus[a.UserID] {
			continue
		}
		vus[a.UserID] = true
		if err := s.tournerCompte(ctx, a.UserID, maintenant); err != nil {
			slog.Error("notifications : compte", "compte", a.UserID, "err", err)
		}
	}
	return nil
}

func (s *Service) tournerCompte(ctx context.Context, uid int64, maintenant time.Time) error {
	p, err := s.Donnees.PreferencesNotifications(ctx, uid)
	if err != nil || (!p.Soiree && !p.RendezVous) {
		return err
	}
	a, m, j := maintenant.In(Paris).Date()
	jour := time.Date(a, m, j, 0, 0, 0, 0, time.UTC)
	evs, err := s.Agenda(ctx, uid, jour, jour.AddDate(0, 0, 1))
	if err != nil {
		return err
	}
	for _, n := range Choisir(evs, p, maintenant) {
		libre, err := s.Donnees.ReserverNotification(ctx, uid, n.Cle)
		if err != nil {
			return err
		}
		if !libre {
			continue
		}
		// Aucun appareil atteint : on réessaie à la minute suivante, tant que la fenêtre dure.
		if atteints, err := s.Pousser(ctx, uid, n); atteints == 0 {
			_ = s.Donnees.AnnulerNotification(ctx, uid, n.Cle)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Planifier passe au début de chaque minute, et oublie chaque jour les envois anciens.
func (s *Service) Planifier(ctx context.Context) {
	purge := time.Time{}
	for {
		maintenant := time.Now()
		if maintenant.Sub(purge) > 24*time.Hour {
			if err := s.Donnees.PurgerNotifications(ctx); err != nil {
				slog.Error("notifications : purge", "err", err)
			}
			purge = maintenant
		}
		passage, annuler := context.WithTimeout(ctx, 50*time.Second)
		if err := s.Tourner(passage, maintenant); err != nil {
			slog.Error("notifications", "err", err)
		}
		annuler()
		suivant := maintenant.Truncate(time.Minute).Add(time.Minute + 2*time.Second)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(suivant)):
		}
	}
}
