package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// CleVAPID renvoie la clé privée VAPID du serveur ; creer en fabrique une la première fois.
// Deux API qui démarrent ensemble gardent la même : la première écrite gagne.
func (s *Store) CleVAPID(ctx context.Context, creer func() ([]byte, error)) ([]byte, error) {
	var prive []byte
	err := s.db.QueryRow(ctx, `SELECT prive FROM push_cle`).Scan(&prive)
	if err == nil {
		return prive, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	neuve, err := creer()
	if err != nil {
		return nil, err
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO push_cle (prive) VALUES ($1) ON CONFLICT DO NOTHING`, neuve); err != nil {
		return nil, err
	}
	err = s.db.QueryRow(ctx, `SELECT prive FROM push_cle`).Scan(&prive)
	return prive, err
}

// AbonnementPush est un appareil abonné aux notifications.
type AbonnementPush struct {
	ID       int64     `json:"id"`
	UserID   int64     `json:"-"`
	Endpoint string    `json:"-"`
	P256dh   string    `json:"-"`
	Auth     string    `json:"-"`
	Appareil string    `json:"appareil"`
	CreeLe   time.Time `json:"cree_le"`
	Erreur   string    `json:"erreur,omitempty"`
}

// Abonner enregistre un appareil. Le même navigateur qui se réabonne (ou change de compte)
// remplace son ancienne ligne.
func (s *Store) Abonner(ctx context.Context, a AbonnementPush) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `INSERT INTO abonnements_push (user_id, endpoint, p256dh, auth, appareil) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (endpoint) DO UPDATE SET user_id = $1, p256dh = $3, auth = $4, appareil = $5, erreur = '', created_at = now()
		RETURNING id`, a.UserID, a.Endpoint, a.P256dh, a.Auth, a.Appareil).Scan(&id)
	return id, err
}

// Desabonner retire un appareil, par son adresse de push.
func (s *Store) Desabonner(ctx context.Context, userID int64, endpoint string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM abonnements_push WHERE user_id = $1 AND endpoint = $2`, userID, endpoint)
	return err
}

// SupprimerAbonnement retire un appareil de la liste.
func (s *Store) SupprimerAbonnement(ctx context.Context, userID, id int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM abonnements_push WHERE user_id = $1 AND id = $2`, userID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// OublierAbonnement retire un appareil que son service de push ne connaît plus.
func (s *Store) OublierAbonnement(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM abonnements_push WHERE id = $1`, id)
	return err
}

// NoterErreurAbonnement garde la dernière erreur d'envoi d'un appareil (vide : envoi réussi).
func (s *Store) NoterErreurAbonnement(ctx context.Context, id int64, msg string) error {
	_, err := s.db.Exec(ctx, `UPDATE abonnements_push SET erreur = $2 WHERE id = $1`, id, msg)
	return err
}

// Abonnements liste les appareils d'un compte ; userID 0 : tous les comptes.
func (s *Store) Abonnements(ctx context.Context, userID int64) ([]AbonnementPush, error) {
	rows, err := s.db.Query(ctx, `SELECT id, user_id, endpoint, p256dh, auth, appareil, created_at, erreur
		FROM abonnements_push WHERE $1 = 0 OR user_id = $1 ORDER BY user_id, created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []AbonnementPush{}
	for rows.Next() {
		var a AbonnementPush
		if err := rows.Scan(&a.ID, &a.UserID, &a.Endpoint, &a.P256dh, &a.Auth, &a.Appareil, &a.CreeLe, &a.Erreur); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, rows.Err()
}

// PreferencesNotifications : ce que la personne veut recevoir.
type PreferencesNotifications struct {
	Soiree      bool   `json:"soiree"`       // résumé de « Ta soirée »
	HeureSoiree string `json:"heure_soiree"` // HH:MM, heure de Paris
	RendezVous  bool   `json:"rendez_vous"`  // rappel avant chaque rendez-vous à heure fixe
	Avance      int    `json:"avance"`       // minutes avant le rendez-vous
}

// PreferencesNotificationsDefaut sert tant que la personne n'a rien changé : tout est coupé,
// c'est à elle de choisir ce qu'elle veut recevoir.
var PreferencesNotificationsDefaut = PreferencesNotifications{Soiree: false, HeureSoiree: "18:00", RendezVous: false, Avance: 15}

func (s *Store) PreferencesNotifications(ctx context.Context, userID int64) (PreferencesNotifications, error) {
	p := PreferencesNotificationsDefaut
	err := s.db.QueryRow(ctx, `SELECT soiree, to_char(heure_soiree, 'HH24:MI'), rendez_vous, avance
		FROM preferences_notifications WHERE user_id = $1`, userID).Scan(&p.Soiree, &p.HeureSoiree, &p.RendezVous, &p.Avance)
	if errors.Is(err, pgx.ErrNoRows) {
		return PreferencesNotificationsDefaut, nil
	}
	return p, err
}

func (s *Store) ChangerPreferencesNotifications(ctx context.Context, userID int64, p PreferencesNotifications) error {
	_, err := s.db.Exec(ctx, `INSERT INTO preferences_notifications (user_id, soiree, heure_soiree, rendez_vous, avance)
		VALUES ($1, $2, $3::time, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET soiree = $2, heure_soiree = $3::time, rendez_vous = $4, avance = $5, updated_at = now()`,
		userID, p.Soiree, p.HeureSoiree, p.RendezVous, p.Avance)
	return err
}

// ReserverNotification enregistre un envoi avant qu'il parte ; faux s'il est déjà parti.
func (s *Store) ReserverNotification(ctx context.Context, userID int64, cle string) (bool, error) {
	tag, err := s.db.Exec(ctx, `INSERT INTO notifications_envoyees (user_id, cle) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, cle)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// AnnulerNotification retire la réservation d'un envoi qui n'a atteint aucun appareil.
func (s *Store) AnnulerNotification(ctx context.Context, userID int64, cle string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM notifications_envoyees WHERE user_id = $1 AND cle = $2`, userID, cle)
	return err
}

// PurgerNotifications oublie les envois de plus d'un mois : leur date est passée.
func (s *Store) PurgerNotifications(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `DELETE FROM notifications_envoyees WHERE envoyee_le < now() - interval '35 days'`)
	return err
}
