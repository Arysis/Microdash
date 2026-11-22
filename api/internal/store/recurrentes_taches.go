package store

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// SourceRecurrente marque les tâches créées par une tâche récurrente (source_ref : « id:AAAA-MM-JJ »).
const SourceRecurrente = "recurrente"

// TacheRecurrente est le modèle d'une tâche qui revient.
type TacheRecurrente struct {
	ID           int64  `json:"id"`
	Titre        string `json:"titre"`
	Notes        string `json:"notes"`
	Projet       string `json:"projet"`
	Duree        int    `json:"duree_minutes,omitempty"`
	Rythme       string `json:"rythme"` // jour, semaine, mois
	Depuis       string `json:"depuis"` // AAAA-MM-JJ : première date, qui donne le jour de la semaine ou du mois
	GenereJusqua string `json:"-"`
}

const colonnesRecurrente = `id, titre, notes, projet, COALESCE(duree_minutes, 0), rythme, to_char(depuis, 'YYYY-MM-DD'),
	COALESCE(to_char(genere_jusqua, 'YYYY-MM-DD'), '')`

func scanRecurrente(row pgx.Row) (*TacheRecurrente, error) {
	r := &TacheRecurrente{}
	err := row.Scan(&r.ID, &r.Titre, &r.Notes, &r.Projet, &r.Duree, &r.Rythme, &r.Depuis, &r.GenereJusqua)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return r, err
}

// TachesRecurrentes liste les modèles d'un compte ; userID 0 : tous les comptes.
func (s *Store) TachesRecurrentes(ctx context.Context, userID int64) ([]TacheRecurrente, []int64, error) {
	rows, err := s.db.Query(ctx, `SELECT `+colonnesRecurrente+`, user_id FROM taches_recurrentes
		WHERE $1 = 0 OR user_id = $1 ORDER BY user_id, rythme, titre, id`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	res, comptes := []TacheRecurrente{}, []int64{}
	for rows.Next() {
		var r TacheRecurrente
		var uid int64
		if err := rows.Scan(&r.ID, &r.Titre, &r.Notes, &r.Projet, &r.Duree, &r.Rythme, &r.Depuis, &r.GenereJusqua, &uid); err != nil {
			return nil, nil, err
		}
		res, comptes = append(res, r), append(comptes, uid)
	}
	return res, comptes, rows.Err()
}

func (s *Store) CreateTacheRecurrente(ctx context.Context, userID int64, r TacheRecurrente) (*TacheRecurrente, error) {
	return scanRecurrente(s.db.QueryRow(ctx, `INSERT INTO taches_recurrentes (user_id, titre, notes, projet, duree_minutes, rythme, depuis)
		VALUES ($1, $2, $3, $4, NULLIF($5, 0), $6, $7::date) RETURNING `+colonnesRecurrente,
		userID, r.Titre, r.Notes, r.Projet, r.Duree, r.Rythme, r.Depuis))
}

// ArreterTacheRecurrente supprime le modèle et les tâches qu'il a créées après aujourd'hui et
// pas encore faites. Celles d'aujourd'hui et d'avant restent.
func (s *Store) ArreterTacheRecurrente(ctx context.Context, userID, id int64, aujourdhui time.Time) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `DELETE FROM taches_recurrentes WHERE id = $1 AND user_id = $2`, id, userID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		_, err = tx.Exec(ctx, `DELETE FROM taches WHERE user_id = $1 AND source = $2 AND source_ref LIKE $3
			AND faite_le IS NULL AND debut > $4`, userID, SourceRecurrente, strconv.FormatInt(id, 10)+":%", aujourdhui)
		return err
	})
}

// CreerOccurrences crée les tâches d'un modèle pour les dates données, et retient la dernière
// date traitée : un second passage ne recrée rien.
func (s *Store) CreerOccurrences(ctx context.Context, userID int64, r TacheRecurrente, dates []string, jusqua string) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		// Verrou : deux passages simultanés ne créent pas deux fois la même date.
		var deja string
		err := tx.QueryRow(ctx, `SELECT COALESCE(to_char(genere_jusqua, 'YYYY-MM-DD'), '') FROM taches_recurrentes
			WHERE id = $1 AND user_id = $2 FOR UPDATE`, r.ID, userID).Scan(&deja)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // arrêtée entre-temps
		}
		if err != nil {
			return err
		}
		for _, d := range dates {
			if deja != "" && d <= deja {
				continue
			}
			if _, err := tx.Exec(ctx, `INSERT INTO taches (user_id, titre, notes, projet, duree_minutes, quand, debut, fin, source, source_ref)
				VALUES ($1, $2, $3, $4, NULLIF($5, 0), 'dates', $6::date, $6::date, $7, $8)`,
				userID, r.Titre, r.Notes, r.Projet, r.Duree, d, SourceRecurrente, strconv.FormatInt(r.ID, 10)+":"+d); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE taches_recurrentes SET genere_jusqua = GREATEST(COALESCE(genere_jusqua, $2::date), $2::date) WHERE id = $1`, r.ID, jusqua)
		return err
	})
}

// TachesBilan renvoie les tâches faites entre deux dates, ou dont la période finit entre elles.
func (s *Store) TachesBilan(ctx context.Context, userID int64, du, au time.Time) ([]Tache, error) {
	return s.listerTaches(ctx, `WHERE user_id = $1 AND (faite_le BETWEEN $2 AND $3 OR fin BETWEEN $2 AND $3)
		ORDER BY COALESCE(faite_le, fin), id`, userID, du, au)
}
