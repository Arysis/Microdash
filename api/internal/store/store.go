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

	"github.com/arysis/microdash/api/internal/calc"
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
	// Recurrente : saisie créée par une dépense récurrente.
	Recurrente bool `json:"recurrente"`
}

const colonnesTx = `id, type, date, centimes, categorie, poste, libelle, tiers, recurrente_id IS NOT NULL`

func scanTx(row pgx.Row) (*Transaction, error) {
	t := &Transaction{}
	err := row.Scan(&t.ID, &t.Type, &t.Date, &t.Centimes, &t.Categorie, &t.Poste, &t.Libelle, &t.Tiers, &t.Recurrente)
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

// --- Agenda ---

// EcheancesFaites renvoie les codes d'échéances cochés, avec leur date.
func (s *Store) EcheancesFaites(ctx context.Context, userID int64) (map[string]time.Time, error) {
	rows, err := s.db.Query(ctx, `SELECT code, faite_le FROM echeances_faites WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := map[string]time.Time{}
	for rows.Next() {
		var code string
		var le time.Time
		if err := rows.Scan(&code, &le); err != nil {
			return nil, err
		}
		res[code] = le
	}
	return res, rows.Err()
}

func (s *Store) MarquerEcheance(ctx context.Context, userID int64, code string, faite bool) error {
	var err error
	if faite {
		_, err = s.db.Exec(ctx, `INSERT INTO echeances_faites (user_id, code) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, code)
	} else {
		_, err = s.db.Exec(ctx, `DELETE FROM echeances_faites WHERE user_id = $1 AND code = $2`, userID, code)
	}
	return err
}

// --- Alertes ---

type Preferences struct {
	Echeances bool `json:"echeances"`
	Plafond   bool `json:"plafond"`
	TVA       bool `json:"tva"`
	CFE       bool `json:"cfe"`
}

var PreferencesParDefaut = Preferences{Echeances: true, Plafond: true, TVA: true, CFE: true}

func (s *Store) Preferences(ctx context.Context, userID int64) (Preferences, error) {
	p := PreferencesParDefaut
	err := s.db.QueryRow(ctx, `SELECT echeances, plafond, tva, cfe FROM preferences_alertes WHERE user_id = $1`, userID).
		Scan(&p.Echeances, &p.Plafond, &p.TVA, &p.CFE)
	if errors.Is(err, pgx.ErrNoRows) {
		return PreferencesParDefaut, nil
	}
	return p, err
}

func (s *Store) SavePreferences(ctx context.Context, userID int64, p Preferences) error {
	_, err := s.db.Exec(ctx, `INSERT INTO preferences_alertes (user_id, echeances, plafond, tva, cfe) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET echeances = $2, plafond = $3, tva = $4, cfe = $5, updated_at = now()`,
		userID, p.Echeances, p.Plafond, p.TVA, p.CFE)
	return err
}

// ReserverAlerte enregistre un envoi avant qu'il parte ; faux si la même alerte est déjà partie.
func (s *Store) ReserverAlerte(ctx context.Context, userID int64, cle string) (bool, error) {
	tag, err := s.db.Exec(ctx, `INSERT INTO alertes_envoyees (user_id, cle) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, cle)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// AnnulerAlerte retire la réservation d'un envoi qui a échoué, pour réessayer au prochain passage.
func (s *Store) AnnulerAlerte(ctx context.Context, userID int64, cle string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM alertes_envoyees WHERE user_id = $1 AND cle = $2`, userID, cle)
	return err
}

// Destinataire est un compte avec un profil, tel que le lit la tâche des rappels.
type Destinataire struct {
	ID          int64
	Email       string
	Profil      Profil
	Preferences Preferences
}

// Destinataires liste les comptes qui ont un profil et au moins un rappel activé.
func (s *Store) Destinataires(ctx context.Context) ([]Destinataire, error) {
	rows, err := s.db.Query(ctx, `SELECT u.id, u.email,
			p.categorie, p.categorie_secondaire, p.nature_cfp, p.debut_activite, p.acre, p.versement_liberatoire, p.periodicite,
			COALESCE(a.echeances, true), COALESCE(a.plafond, true), COALESCE(a.tva, true), COALESCE(a.cfe, true)
		FROM users u
		JOIN LATERAL (SELECT * FROM profils WHERE user_id = u.id ORDER BY id DESC LIMIT 1) p ON true
		LEFT JOIN preferences_alertes a ON a.user_id = u.id
		ORDER BY u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Destinataire
	for rows.Next() {
		var d Destinataire
		p, a := &d.Profil, &d.Preferences
		if err := rows.Scan(&d.ID, &d.Email, &p.Categorie, &p.CategorieSecondaire, &p.NatureCFP, &p.DebutActivite, &p.ACRE, &p.VersementLiberatoire, &p.Periodicite,
			&a.Echeances, &a.Plafond, &a.TVA, &a.CFE); err != nil {
			return nil, err
		}
		if a.Echeances || a.Plafond || a.TVA || a.CFE {
			res = append(res, d)
		}
	}
	return res, rows.Err()
}

// VersCalc donne le profil sous la forme qu'attend le calcul.
func (p Profil) VersCalc() calc.Profil {
	return calc.Profil{
		Categorie: p.Categorie, CategorieSecondaire: p.CategorieSecondaire, NatureCFP: p.NatureCFP,
		DebutActivite: p.DebutActivite, ACRE: p.ACRE, VersementLiberatoire: p.VersementLiberatoire,
	}
}

// VersCalc donne les transactions sous la forme qu'attend le calcul.
func VersCalc(txs []Transaction) []calc.Transaction {
	res := make([]calc.Transaction, 0, len(txs))
	for _, t := range txs {
		res = append(res, calc.Transaction{Type: t.Type, Date: t.Date, Centimes: t.Centimes, Categorie: t.Categorie})
	}
	return res
}

// --- Dépenses récurrentes ---

type Recurrente struct {
	ID        int64      `json:"id"`
	Libelle   string     `json:"libelle"`
	Poste     string     `json:"poste"`
	Centimes  int64      `json:"centimes"`
	Frequence string     `json:"frequence"`
	Debut     time.Time  `json:"-"`
	Fin       *time.Time `json:"-"`
}

func (r Recurrente) VersCalc() calc.Recurrente {
	return calc.Recurrente{Centimes: r.Centimes, Frequence: r.Frequence, Debut: r.Debut, Fin: r.Fin}
}

const colonnesRec = `id, libelle, poste, centimes, frequence, debut, fin`

func scanRec(row pgx.Row) (*Recurrente, error) {
	r := &Recurrente{}
	err := row.Scan(&r.ID, &r.Libelle, &r.Poste, &r.Centimes, &r.Frequence, &r.Debut, &r.Fin)
	return r, err
}

func (s *Store) Recurrentes(ctx context.Context, userID int64) ([]Recurrente, error) {
	rows, err := s.db.Query(ctx, `SELECT `+colonnesRec+` FROM depenses_recurrentes WHERE user_id = $1 ORDER BY libelle, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []Recurrente{}
	for rows.Next() {
		r, err := scanRec(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *r)
	}
	return res, rows.Err()
}

func (s *Store) CreateRecurrente(ctx context.Context, userID int64, r Recurrente) (*Recurrente, error) {
	return scanRec(s.db.QueryRow(ctx, `INSERT INTO depenses_recurrentes (user_id, libelle, poste, centimes, frequence, debut, fin)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+colonnesRec,
		userID, r.Libelle, r.Poste, r.Centimes, r.Frequence, r.Debut, r.Fin))
}

// UpdateRecurrente change une dépense récurrente ; les saisies déjà créées ne bougent pas.
func (s *Store) UpdateRecurrente(ctx context.Context, userID int64, r Recurrente) (*Recurrente, error) {
	res, err := scanRec(s.db.QueryRow(ctx, `UPDATE depenses_recurrentes SET libelle = $3, poste = $4, centimes = $5, frequence = $6, debut = $7, fin = $8
		WHERE id = $1 AND user_id = $2 RETURNING `+colonnesRec,
		r.ID, userID, r.Libelle, r.Poste, r.Centimes, r.Frequence, r.Debut, r.Fin))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return res, err
}

// DeleteRecurrente arrête une dépense récurrente ; les saisies déjà créées restent.
func (s *Store) DeleteRecurrente(ctx context.Context, userID, id int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM depenses_recurrentes WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GenererRecurrentes crée les saisies des dépenses récurrentes jusqu'à la date donnée incluse,
// pour un compte (userID > 0) ou pour tous (userID = 0). Une date déjà traitée ne l'est plus :
// une saisie supprimée à la main n'est pas recréée. Renvoie le nombre de saisies créées.
func (s *Store) GenererRecurrentes(ctx context.Context, userID int64, jusqua time.Time) (int, error) {
	cree := 0
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+colonnesRec+`, user_id, genere_jusqua FROM depenses_recurrentes
			WHERE ($1 = 0 OR user_id = $1) AND debut <= $2 AND (genere_jusqua IS NULL OR genere_jusqua < $2)
			FOR UPDATE`, userID, jusqua)
		if err != nil {
			return err
		}
		type aFaire struct {
			r      Recurrente
			uid    int64
			depuis time.Time
		}
		var liste []aFaire
		for rows.Next() {
			var a aFaire
			var genere *time.Time
			if err := rows.Scan(&a.r.ID, &a.r.Libelle, &a.r.Poste, &a.r.Centimes, &a.r.Frequence, &a.r.Debut, &a.r.Fin, &a.uid, &genere); err != nil {
				rows.Close()
				return err
			}
			a.depuis = a.r.Debut
			if genere != nil {
				a.depuis = genere.AddDate(0, 0, 1)
			}
			liste = append(liste, a)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, a := range liste {
			for _, d := range a.r.VersCalc().Occurrences(a.depuis, jusqua) {
				if _, err := tx.Exec(ctx, `INSERT INTO transactions (user_id, type, date, centimes, poste, libelle, recurrente_id)
					VALUES ($1, 'depense', $2, $3, $4, $5, $6)`, a.uid, d, a.r.Centimes, a.r.Poste, a.r.Libelle, a.r.ID); err != nil {
					return err
				}
				cree++
			}
			if _, err := tx.Exec(ctx, `UPDATE depenses_recurrentes SET genere_jusqua = $2 WHERE id = $1`, a.r.ID, jusqua); err != nil {
				return err
			}
		}
		return nil
	})
	return cree, err
}

// --- Trésorerie ---

type Solde struct {
	Centimes int64     `json:"centimes"`
	Au       time.Time `json:"-"`
}

func (s *Store) Solde(ctx context.Context, userID int64) (*Solde, error) {
	so := &Solde{}
	err := s.db.QueryRow(ctx, `SELECT centimes, au FROM soldes_tresorerie WHERE user_id = $1`, userID).Scan(&so.Centimes, &so.Au)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return so, err
}

func (s *Store) SaveSolde(ctx context.Context, userID int64, so *Solde) error {
	var err error
	if so == nil {
		_, err = s.db.Exec(ctx, `DELETE FROM soldes_tresorerie WHERE user_id = $1`, userID)
	} else {
		_, err = s.db.Exec(ctx, `INSERT INTO soldes_tresorerie (user_id, centimes, au) VALUES ($1, $2, $3)
			ON CONFLICT (user_id) DO UPDATE SET centimes = $2, au = $3`, userID, so.Centimes, so.Au)
	}
	return err
}

// --- Stripe ---

type ConnexionStripe struct {
	Cle       []byte // chiffrée
	CleFin    string
	Mode      string
	CompteID  string
	CompteNom string
	Donnees   []byte // JSON de la dernière lecture
	LuLe      *time.Time
	Erreur    string
	Depuis    time.Time
	// PrevisionMRR : la prévision prend le MRR comme recettes de chaque mois.
	PrevisionMRR bool
}

func (s *Store) ConnexionStripe(ctx context.Context, userID int64) (*ConnexionStripe, error) {
	c := &ConnexionStripe{}
	err := s.db.QueryRow(ctx, `SELECT cle, cle_fin, mode, compte_id, compte_nom, donnees, lu_le, erreur, created_at, prevision_mrr
		FROM stripe_connexions WHERE user_id = $1`, userID).
		Scan(&c.Cle, &c.CleFin, &c.Mode, &c.CompteID, &c.CompteNom, &c.Donnees, &c.LuLe, &c.Erreur, &c.Depuis, &c.PrevisionMRR)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

func (s *Store) SaveConnexionStripe(ctx context.Context, userID int64, c ConnexionStripe) error {
	_, err := s.db.Exec(ctx, `INSERT INTO stripe_connexions (user_id, cle, cle_fin, mode, compte_id, compte_nom, donnees, lu_le, erreur)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '')
		ON CONFLICT (user_id) DO UPDATE SET cle = $2, cle_fin = $3, mode = $4, compte_id = $5, compte_nom = $6, donnees = $7, lu_le = $8, erreur = '', created_at = now()`,
		userID, c.Cle, c.CleFin, c.Mode, c.CompteID, c.CompteNom, c.Donnees, c.LuLe)
	return err
}

// MajLectureStripe enregistre une nouvelle lecture réussie, ou l'erreur de la dernière tentative.
func (s *Store) MajLectureStripe(ctx context.Context, userID int64, donnees []byte, erreur string) error {
	var err error
	if erreur != "" {
		_, err = s.db.Exec(ctx, `UPDATE stripe_connexions SET erreur = $2 WHERE user_id = $1`, userID, erreur)
	} else {
		_, err = s.db.Exec(ctx, `UPDATE stripe_connexions SET donnees = $2, lu_le = now(), erreur = '' WHERE user_id = $1`, userID, donnees)
	}
	return err
}

// PrevisionStripe choisit si la prévision suit le MRR Stripe ou la moyenne des saisies.
func (s *Store) PrevisionStripe(ctx context.Context, userID int64, mrr bool) error {
	tag, err := s.db.Exec(ctx, `UPDATE stripe_connexions SET prevision_mrr = $2 WHERE user_id = $1`, userID, mrr)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteConnexionStripe(ctx context.Context, userID int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM stripe_connexions WHERE user_id = $1`, userID)
	return err
}
