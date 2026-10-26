package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Evenement est un rendez-vous du module Agenda. Jour au format AAAA-MM-JJ, Heure en HH:MM
// (heure de Paris, vide pour toute la journée).
type Evenement struct {
	ID    int64  `json:"id"`
	Titre string `json:"titre"`
	Jour  string `json:"jour"`
	Heure string `json:"heure,omitempty"`
	Duree int    `json:"duree_minutes,omitempty"`
	Notes string `json:"notes"`
}

const colonnesEvenement = `id, titre, to_char(jour, 'YYYY-MM-DD'), COALESCE(to_char(heure, 'HH24:MI'), ''), COALESCE(duree_minutes, 0), notes`

func scanEvenement(row pgx.Row) (*Evenement, error) {
	e := &Evenement{}
	err := row.Scan(&e.ID, &e.Titre, &e.Jour, &e.Heure, &e.Duree, &e.Notes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return e, err
}

// Evenements renvoie les rendez-vous entre deux jours inclus, dans l'ordre.
func (s *Store) Evenements(ctx context.Context, userID int64, du, au time.Time) ([]Evenement, error) {
	rows, err := s.db.Query(ctx, `SELECT `+colonnesEvenement+` FROM evenements
		WHERE user_id = $1 AND jour BETWEEN $2 AND $3 ORDER BY jour, heure NULLS FIRST, id`, userID, du, au)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []Evenement{}
	for rows.Next() {
		e, err := scanEvenement(rows)
		if err != nil {
			return nil, err
		}
		res = append(res, *e)
	}
	return res, rows.Err()
}

func (s *Store) CreateEvenement(ctx context.Context, userID int64, e Evenement) (*Evenement, error) {
	return scanEvenement(s.db.QueryRow(ctx, `INSERT INTO evenements (user_id, titre, jour, heure, duree_minutes, notes)
		VALUES ($1, $2, $3::date, NULLIF($4, '')::time, NULLIF($5, 0), $6) RETURNING `+colonnesEvenement,
		userID, e.Titre, e.Jour, e.Heure, e.Duree, e.Notes))
}

func (s *Store) UpdateEvenement(ctx context.Context, userID int64, e Evenement) (*Evenement, error) {
	return scanEvenement(s.db.QueryRow(ctx, `UPDATE evenements SET titre = $3, jour = $4::date, heure = NULLIF($5, '')::time,
		duree_minutes = NULLIF($6, 0), notes = $7 WHERE id = $1 AND user_id = $2 RETURNING `+colonnesEvenement,
		e.ID, userID, e.Titre, e.Jour, e.Heure, e.Duree, e.Notes))
}

func (s *Store) DeleteEvenement(ctx context.Context, userID, id int64) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM evenements WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// FluxICal dit depuis quand le lien du flux iCal existe (nil : pas de lien).
func (s *Store) FluxICal(ctx context.Context, userID int64) (*time.Time, error) {
	var t time.Time
	err := s.db.QueryRow(ctx, `SELECT created_at FROM flux_ical WHERE user_id = $1`, userID).Scan(&t)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &t, err
}

// ChangerFluxICal remplace le lien du flux iCal : l'ancien cesse de marcher.
func (s *Store) ChangerFluxICal(ctx context.Context, userID int64, empreinte []byte) error {
	_, err := s.db.Exec(ctx, `INSERT INTO flux_ical (user_id, empreinte) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET empreinte = $2, created_at = now()`, userID, empreinte)
	return err
}

func (s *Store) SupprimerFluxICal(ctx context.Context, userID int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM flux_ical WHERE user_id = $1`, userID)
	return err
}

// CompteDuFlux renvoie le compte à qui appartient un lien de flux iCal.
func (s *Store) CompteDuFlux(ctx context.Context, empreinte []byte) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `SELECT user_id FROM flux_ical WHERE empreinte = $1`, empreinte).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}
