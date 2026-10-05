// Package store accède à la base PostgreSQL.
package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

var ErrNotFound = errors.New("introuvable")

type Store struct{ db *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	// La base peut démarrer après l'API dans docker compose : on attend un peu.
	for i := 0; ; i++ {
		if err = db.Ping(ctx); err == nil {
			break
		}
		if i == 30 {
			db.Close()
			return nil, fmt.Errorf("base injoignable : %w", err)
		}
		time.Sleep(time.Second)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() { s.db.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.db.Ping(ctx) }

// Migrate applique, dans l'ordre, les fichiers SQL embarqués qui ne l'ont pas encore été.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (name TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	noms, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(noms)
	for _, nom := range noms {
		var deja bool
		if err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)`, nom).Scan(&deja); err != nil {
			return err
		}
		if deja {
			continue
		}
		sql, err := migrations.ReadFile(nom)
		if err != nil {
			return err
		}
		err = pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(sql)); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, nom)
			return err
		})
		if err != nil {
			return fmt.Errorf("migration %s : %w", nom, err)
		}
	}
	return nil
}

// --- Comptes et sessions ---

type User struct {
	ID           int64
	Email        string
	PasswordHash string
}

var ErrEmailPris = errors.New("adresse e-mail déjà utilisée")

func (s *Store) CreateUser(ctx context.Context, email, hash string) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `INSERT INTO users (email, password_hash) VALUES ($1, $2) ON CONFLICT (email) DO NOTHING RETURNING id`, email, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrEmailPris
	}
	return id, err
}

func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(ctx, `SELECT id, email, password_hash FROM users WHERE email = $1`, email).Scan(&u.ID, &u.Email, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) UserByID(ctx context.Context, id int64) (*User, error) {
	u := &User{}
	err := s.db.QueryRow(ctx, `SELECT id, email, password_hash FROM users WHERE id = $1`, id).Scan(&u.ID, &u.Email, &u.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return u, err
}

func (s *Store) DeleteUser(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	return err
}

func (s *Store) CreateSession(ctx context.Context, tokenHash []byte, userID int64, expires time.Time) error {
	_, err := s.db.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`, tokenHash, userID, expires)
	return err
}

func (s *Store) SessionUser(ctx context.Context, tokenHash []byte) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `SELECT user_id FROM sessions WHERE token_hash = $1 AND expires_at > now()`, tokenHash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

func (s *Store) DeleteSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.db.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1 OR expires_at < now()`, tokenHash)
	return err
}

// --- Profil ---

type Profil struct {
	Categorie            string    `json:"categorie"`
	CategorieSecondaire  string    `json:"categorie_secondaire"`
	NatureCFP            string    `json:"nature_cfp"`
	DebutActivite        time.Time `json:"-"`
	ACRE                 bool      `json:"acre"`
	VersementLiberatoire bool      `json:"versement_liberatoire"`
	Periodicite          string    `json:"periodicite"`
}

func (s *Store) Profil(ctx context.Context, userID int64) (*Profil, error) {
	p := &Profil{}
	err := s.db.QueryRow(ctx, `SELECT categorie, categorie_secondaire, nature_cfp, debut_activite, acre, versement_liberatoire, periodicite
		FROM profils WHERE user_id = $1 ORDER BY id DESC LIMIT 1`, userID).
		Scan(&p.Categorie, &p.CategorieSecondaire, &p.NatureCFP, &p.DebutActivite, &p.ACRE, &p.VersementLiberatoire, &p.Periodicite)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return p, err
}

func (s *Store) SaveProfil(ctx context.Context, userID int64, p Profil) error {
	_, err := s.db.Exec(ctx, `INSERT INTO profils (user_id, categorie, categorie_secondaire, nature_cfp, debut_activite, acre, versement_liberatoire, periodicite)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		userID, p.Categorie, p.CategorieSecondaire, p.NatureCFP, p.DebutActivite, p.ACRE, p.VersementLiberatoire, p.Periodicite)
	return err
}

// --- Transactions ---

type Transaction struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Date      time.Time `json:"-"`
	Centimes  int64     `json:"centimes"`
	Categorie string    `json:"categorie"`
	Poste     string    `json:"poste"`
	Libelle   string    `json:"libelle"`
	Tiers     string    `json:"tiers"`
}

const colonnesTx = `id, type, date, centimes, categorie, poste, libelle, tiers`

func scanTx(row pgx.Row) (*Transaction, error) {
	t := &Transaction{}
	err := row.Scan(&t.ID, &t.Type, &t.Date, &t.Centimes, &t.Categorie, &t.Poste, &t.Libelle, &t.Tiers)
	return t, err
}

// Transactions renvoie les transactions de [du, au], les plus récentes d'abord.
func (s *Store) Transactions(ctx context.Context, userID int64, du, au time.Time) ([]Transaction, error) {
	rows, err := s.db.Query(ctx, `SELECT `+colonnesTx+` FROM transactions WHERE user_id = $1 AND date BETWEEN $2 AND $3 ORDER BY date DESC, id DESC`, userID, du, au)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []Transaction{}
	for rows.Next() {
		t, err := scanTx(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *t)
	}
	return res, rows.Err()
}

func (s *Store) CreateTransaction(ctx context.Context, userID int64, t Transaction) (*Transaction, error) {
	row := s.db.QueryRow(ctx, `INSERT INTO transactions (user_id, type, date, centimes, categorie, poste, libelle, tiers)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING `+colonnesTx,
		userID, t.Type, t.Date, t.Centimes, t.Categorie, t.Poste, t.Libelle, t.Tiers)
	return scanTx(row)
}

func (s *Store) UpdateTransaction(ctx context.Context, userID int64, t Transaction) (*Transaction, error) {
	row := s.db.QueryRow(ctx, `UPDATE transactions SET type = $3, date = $4, centimes = $5, categorie = $6, poste = $7, libelle = $8, tiers = $9
		WHERE id = $1 AND user_id = $2 RETURNING `+colonnesTx,
		t.ID, userID, t.Type, t.Date, t.Centimes, t.Categorie, t.Poste, t.Libelle, t.Tiers)
	res, err := scanTx(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return res, err
}

func (s *Store) DeleteTransaction(ctx context.Context, userID, id int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM transactions WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
