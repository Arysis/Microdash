package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Mails (Gmail) ---

type ConnexionMail struct {
	Email          string
	MotDePasse     []byte // chiffré
	ImportantGmail bool
	SynchroLe      *time.Time
	Erreur         string
}

func (s *Store) ConnexionMail(ctx context.Context, userID int64) (*ConnexionMail, error) {
	c := &ConnexionMail{}
	err := s.db.QueryRow(ctx, `SELECT email, mot_de_passe, important_gmail, synchro_le, erreur FROM mails_connexions WHERE user_id = $1`, userID).
		Scan(&c.Email, &c.MotDePasse, &c.ImportantGmail, &c.SynchroLe, &c.Erreur)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

// SaveConnexionMail enregistre l'adresse et le mot de passe ; la copie des mails repart de zéro.
func (s *Store) SaveConnexionMail(ctx context.Context, userID int64, c ConnexionMail) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM mails WHERE user_id = $1`, userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO mails_connexions (user_id, email, mot_de_passe, important_gmail) VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id) DO UPDATE SET email = $2, mot_de_passe = $3, important_gmail = $4, synchro_le = NULL, erreur = '', created_at = now()`,
			userID, c.Email, c.MotDePasse, c.ImportantGmail)
		return err
	})
}

func (s *Store) ImportantGmail(ctx context.Context, userID int64, oui bool) error {
	_, err := s.db.Exec(ctx, `UPDATE mails_connexions SET important_gmail = $2 WHERE user_id = $1`, userID, oui)
	return err
}

// SupprimerConnexionMail oublie la connexion Gmail et la copie des mails.
func (s *Store) SupprimerConnexionMail(ctx context.Context, userID int64) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM mails WHERE user_id = $1`, userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM mails_connexions WHERE user_id = $1`, userID)
		return err
	})
}

// ComptesMail renvoie les comptes qui ont connecté Gmail.
func (s *Store) ComptesMail(ctx context.Context) ([]int64, error) {
	rows, err := s.db.Query(ctx, `SELECT user_id FROM mails_connexions ORDER BY user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		res = append(res, id)
	}
	return res, rows.Err()
}

type Mail struct {
	Ref            string    `json:"ref"`
	MessageID      string    `json:"-"`
	De             string    `json:"expediteur"`
	DeNom          string    `json:"expediteur_nom,omitempty"`
	Sujet          string    `json:"sujet"`
	Extrait        string    `json:"extrait,omitempty"`
	Recu           time.Time `json:"recu_le"`
	Lu             bool      `json:"lu"`
	ImportantGmail bool      `json:"-"`
}

// RemplacerMails remplace la copie des mails par le résultat d'une synchro réussie.
func (s *Store) RemplacerMails(ctx context.Context, userID int64, mails []Mail, le time.Time) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM mails WHERE user_id = $1`, userID); err != nil {
			return err
		}
		for _, m := range mails {
			if _, err := tx.Exec(ctx, `INSERT INTO mails (user_id, ref, message_id, expediteur, expediteur_nom, sujet, extrait, recu_le, lu, important_gmail)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) ON CONFLICT DO NOTHING`,
				userID, m.Ref, m.MessageID, m.De, m.DeNom, m.Sujet, m.Extrait, m.Recu, m.Lu, m.ImportantGmail); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `UPDATE mails_connexions SET synchro_le = $2, erreur = '' WHERE user_id = $1`, userID, le)
		return err
	})
}

func (s *Store) NoterErreurMail(ctx context.Context, userID int64, msg string) error {
	_, err := s.db.Exec(ctx, `UPDATE mails_connexions SET erreur = $2 WHERE user_id = $1`, userID, msg)
	return err
}

const colonnesMail = `ref, message_id, expediteur, expediteur_nom, sujet, extrait, recu_le, lu, important_gmail`

func scanMail(row pgx.Row) (*Mail, error) {
	m := &Mail{}
	err := row.Scan(&m.Ref, &m.MessageID, &m.De, &m.DeNom, &m.Sujet, &m.Extrait, &m.Recu, &m.Lu, &m.ImportantGmail)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return m, err
}

// Mails renvoie la copie des mails, du plus récent au plus ancien.
func (s *Store) Mails(ctx context.Context, userID int64) ([]Mail, error) {
	rows, err := s.db.Query(ctx, `SELECT `+colonnesMail+` FROM mails WHERE user_id = $1 ORDER BY recu_le DESC, ref`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []Mail{}
	for rows.Next() {
		m, err := scanMail(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *m)
	}
	return res, rows.Err()
}

func (s *Store) Mail(ctx context.Context, userID int64, ref string) (*Mail, error) {
	return scanMail(s.db.QueryRow(ctx, `SELECT `+colonnesMail+` FROM mails WHERE user_id = $1 AND ref = $2`, userID, ref))
}

func (s *Store) SupprimerMail(ctx context.Context, userID int64, ref string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM mails WHERE user_id = $1 AND ref = $2`, userID, ref)
	return err
}

type RegleMail struct {
	ID      int64  `json:"id"`
	Motif   string `json:"motif"`
	Libelle string `json:"libelle"`
}

func (s *Store) ReglesMail(ctx context.Context, userID int64) ([]RegleMail, error) {
	rows, err := s.db.Query(ctx, `SELECT id, motif, libelle FROM mails_regles WHERE user_id = $1 ORDER BY motif`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []RegleMail{}
	for rows.Next() {
		var r RegleMail
		if err := rows.Scan(&r.ID, &r.Motif, &r.Libelle); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, rows.Err()
}

// CreateRegleMail ajoute une règle ; une règle qui existe déjà garde son identifiant.
func (s *Store) CreateRegleMail(ctx context.Context, userID int64, r RegleMail) (*RegleMail, error) {
	err := s.db.QueryRow(ctx, `INSERT INTO mails_regles (user_id, motif, libelle) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, motif) DO UPDATE SET libelle = $3 RETURNING id`, userID, r.Motif, r.Libelle).Scan(&r.ID)
	return &r, err
}

func (s *Store) DeleteRegleMail(ctx context.Context, userID, id int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM mails_regles WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
