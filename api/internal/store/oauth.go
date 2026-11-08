package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Connecteur Claude (OAuth) ---

// ClientOAuth est une application qui s'est enregistrée pour demander l'accès (Claude).
type ClientOAuth struct {
	ID           string
	Nom          string
	RedirectURIs []string
	SecretHash   []byte // nil : client public
}

func (s *Store) CreateClientOAuth(ctx context.Context, c ClientOAuth) error {
	_, err := s.db.Exec(ctx, `INSERT INTO oauth_clients (id, nom, redirect_uris, secret_hash) VALUES ($1, $2, $3, $4)`,
		c.ID, c.Nom, c.RedirectURIs, c.SecretHash)
	return err
}

func (s *Store) ClientOAuth(ctx context.Context, id string) (*ClientOAuth, error) {
	c := &ClientOAuth{}
	err := s.db.QueryRow(ctx, `SELECT id, nom, redirect_uris, secret_hash FROM oauth_clients WHERE id = $1`, id).
		Scan(&c.ID, &c.Nom, &c.RedirectURIs, &c.SecretHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// CreateCodeOAuth ouvre un accès pour le compte et le client, et y attache un code à usage unique.
func (s *Store) CreateCodeOAuth(ctx context.Context, userID int64, clientID string, codeHash []byte, redirectURI, challenge string, expire time.Time) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var acces int64
		if err := tx.QueryRow(ctx, `INSERT INTO oauth_acces (user_id, client_id) VALUES ($1, $2) RETURNING id`, userID, clientID).Scan(&acces); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO oauth_codes (code_hash, acces_id, redirect_uri, challenge, expire_le) VALUES ($1, $2, $3, $4, $5)`,
			codeHash, acces, redirectURI, challenge, expire)
		return err
	})
}

// CodeOAuth est un code d'autorisation lu (et consommé) à l'échange contre des jetons.
type CodeOAuth struct {
	AccesID     int64
	ClientID    string
	RedirectURI string
	Challenge   string
	Expire      time.Time
}

// ConsommerCodeOAuth lit le code et le supprime : un code ne sert qu'une fois.
func (s *Store) ConsommerCodeOAuth(ctx context.Context, codeHash []byte) (*CodeOAuth, error) {
	c := &CodeOAuth{}
	err := s.db.QueryRow(ctx, `DELETE FROM oauth_codes o USING oauth_acces a WHERE o.code_hash = $1 AND a.id = o.acces_id
		RETURNING o.acces_id, a.client_id, o.redirect_uri, o.challenge, o.expire_le`, codeHash).
		Scan(&c.AccesID, &c.ClientID, &c.RedirectURI, &c.Challenge, &c.Expire)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

// Genres de jetons OAuth.
const (
	JetonAcces          = "acces"
	JetonRenouvellement = "renouvellement"
)

// CreateJetonsOAuth enregistre une paire de jetons (déjà hachés) pour un accès.
func (s *Store) CreateJetonsOAuth(ctx context.Context, accesID int64, acces []byte, expireAcces time.Time, renouv []byte, expireRenouv time.Time) error {
	_, err := s.db.Exec(ctx, `INSERT INTO oauth_jetons (hash, acces_id, genre, expire_le) VALUES ($1, $2, 'acces', $3), ($4, $2, 'renouvellement', $5)`,
		acces, accesID, expireAcces, renouv, expireRenouv)
	return err
}

// JetonOAuth est un jeton valide retrouvé par son empreinte.
type JetonOAuth struct {
	AccesID  int64
	UserID   int64
	ClientID string
	Expire   time.Time
}

// JetonOAuth renvoie le jeton du genre donné s'il n'a pas expiré, et note l'heure d'usage de son accès.
func (s *Store) JetonOAuth(ctx context.Context, hash []byte, genre string) (*JetonOAuth, error) {
	j := &JetonOAuth{}
	err := s.db.QueryRow(ctx, `UPDATE oauth_acces a SET utilise_le = now() FROM oauth_jetons j
		WHERE j.hash = $1 AND j.genre = $2 AND j.expire_le > now() AND a.id = j.acces_id
		RETURNING a.id, a.user_id, a.client_id, j.expire_le`, hash, genre).Scan(&j.AccesID, &j.UserID, &j.ClientID, &j.Expire)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return j, err
}

// SupprimerJetonOAuth retire un jeton (renouvellement utilisé, ou révocation) et purge les jetons expirés.
func (s *Store) SupprimerJetonOAuth(ctx context.Context, hash []byte) (bool, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM oauth_jetons WHERE hash = $1`, hash)
	if err != nil {
		return false, err
	}
	_, err = s.db.Exec(ctx, `DELETE FROM oauth_jetons WHERE expire_le < now()`)
	return tag.RowsAffected() == 1, err
}

// AccesOAuth est une autorisation donnée par le compte, affichée dans Compte.
type AccesOAuth struct {
	ID        int64      `json:"id"`
	Client    string     `json:"client"`
	CreeLe    time.Time  `json:"cree_le"`
	UtiliseLe *time.Time `json:"utilise_le,omitempty"`
}

// AccesOAuth liste les accès encore ouverts du compte (au moins un jeton ou un code vivant).
func (s *Store) AccesOAuth(ctx context.Context, userID int64) ([]AccesOAuth, error) {
	rows, err := s.db.Query(ctx, `SELECT a.id, c.nom, a.cree_le, a.utilise_le FROM oauth_acces a
		JOIN oauth_clients c ON c.id = a.client_id
		WHERE a.user_id = $1 AND (
			EXISTS (SELECT 1 FROM oauth_jetons j WHERE j.acces_id = a.id AND j.expire_le > now())
			OR EXISTS (SELECT 1 FROM oauth_codes o WHERE o.acces_id = a.id AND o.expire_le > now()))
		ORDER BY a.cree_le DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []AccesOAuth{}
	for rows.Next() {
		var a AccesOAuth
		if err := rows.Scan(&a.ID, &a.Client, &a.CreeLe, &a.UtiliseLe); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, rows.Err()
}

// CouperAccesOAuth supprime tous les accès du compte, et donc leurs jetons.
func (s *Store) CouperAccesOAuth(ctx context.Context, userID int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM oauth_acces WHERE user_id = $1`, userID)
	return err
}

// CouperUnAccesOAuth supprime un accès précis (révocation d'un jeton par le client).
func (s *Store) CouperUnAccesOAuth(ctx context.Context, accesID int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM oauth_acces WHERE id = $1`, accesID)
	return err
}
