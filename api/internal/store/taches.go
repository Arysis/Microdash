package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Tache est une tâche du module Tâches. Les dates sont au format AAAA-MM-JJ, vides si absentes.
type Tache struct {
	ID        int64  `json:"id"`
	Titre     string `json:"titre"`
	Notes     string `json:"notes"`
	Projet    string `json:"projet"`
	Duree     int    `json:"duree_minutes,omitempty"`
	Quand     string `json:"quand"`
	Echeance  string `json:"echeance,omitempty"`
	FaiteLe   string `json:"faite_le,omitempty"`
	Source    string `json:"source,omitempty"`
	SourceRef string `json:"source_ref,omitempty"`
	Lien      string `json:"lien,omitempty"`
}

const colonnesTache = `id, titre, notes, projet, COALESCE(duree_minutes, 0), quand,
	COALESCE(to_char(echeance, 'YYYY-MM-DD'), ''), COALESCE(to_char(faite_le, 'YYYY-MM-DD'), ''), source, source_ref, lien`

func scanTache(row pgx.Row) (*Tache, error) {
	t := &Tache{}
	err := row.Scan(&t.ID, &t.Titre, &t.Notes, &t.Projet, &t.Duree, &t.Quand, &t.Echeance, &t.FaiteLe, &t.Source, &t.SourceRef, &t.Lien)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

func (s *Store) listerTaches(ctx context.Context, requete string, args ...any) ([]Tache, error) {
	rows, err := s.db.Query(ctx, `SELECT `+colonnesTache+` FROM taches `+requete, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []Tache{}
	for rows.Next() {
		t, err := scanTache(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *t)
	}
	return res, rows.Err()
}

// Taches renvoie les tâches à faire, et celles faites depuis la date donnée (pour qu'une tâche
// cochée reste visible, barrée, le temps de la soirée).
func (s *Store) Taches(ctx context.Context, userID int64, faitesDepuis time.Time) ([]Tache, error) {
	return s.listerTaches(ctx, `WHERE user_id = $1 AND (faite_le IS NULL OR faite_le >= $2)
		ORDER BY faite_le NULLS FIRST, echeance NULLS LAST, created_at, id`, userID, faitesDepuis)
}

// TachesDues renvoie les tâches pas faites dont l'échéance est arrivée.
func (s *Store) TachesDues(ctx context.Context, userID int64, jour time.Time) ([]Tache, error) {
	return s.listerTaches(ctx, `WHERE user_id = $1 AND faite_le IS NULL AND echeance <= $2 ORDER BY echeance, id`, userID, jour)
}

// TachesDatees renvoie les tâches dont l'échéance tombe entre deux dates incluses.
func (s *Store) TachesDatees(ctx context.Context, userID int64, du, au time.Time) ([]Tache, error) {
	return s.listerTaches(ctx, `WHERE user_id = $1 AND echeance BETWEEN $2 AND $3 ORDER BY echeance, id`, userID, du, au)
}

func (s *Store) Tache(ctx context.Context, userID, id int64) (*Tache, error) {
	return scanTache(s.db.QueryRow(ctx, `SELECT `+colonnesTache+` FROM taches WHERE id = $1 AND user_id = $2`, id, userID))
}

func (s *Store) CreateTache(ctx context.Context, userID int64, t Tache) (*Tache, error) {
	return scanTache(s.db.QueryRow(ctx, `INSERT INTO taches (user_id, titre, notes, projet, duree_minutes, quand, echeance, source, source_ref, lien)
		VALUES ($1, $2, $3, $4, NULLIF($5, 0), $6, NULLIF($7, '')::date, $8, $9, $10) RETURNING `+colonnesTache,
		userID, t.Titre, t.Notes, t.Projet, t.Duree, t.Quand, t.Echeance, t.Source, t.SourceRef, t.Lien))
}

// UpdateTache change une tâche. faiteLe vide : la tâche est à faire.
func (s *Store) UpdateTache(ctx context.Context, userID int64, t Tache) (*Tache, error) {
	return scanTache(s.db.QueryRow(ctx, `UPDATE taches SET titre = $3, notes = $4, projet = $5, duree_minutes = NULLIF($6, 0),
		quand = $7, echeance = NULLIF($8, '')::date, faite_le = NULLIF($9, '')::date
		WHERE id = $1 AND user_id = $2 RETURNING `+colonnesTache,
		t.ID, userID, t.Titre, t.Notes, t.Projet, t.Duree, t.Quand, t.Echeance, t.FaiteLe))
}

// FinirTache coche une tâche à la date donnée, si elle n'est pas déjà faite.
func (s *Store) FinirTache(ctx context.Context, userID, id int64, jour time.Time) error {
	tag, err := s.db.Exec(ctx, `UPDATE taches SET faite_le = COALESCE(faite_le, $3) WHERE id = $1 AND user_id = $2`, id, userID, jour)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteTache(ctx context.Context, userID, id int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM taches WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
