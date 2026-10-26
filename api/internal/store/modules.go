package store

import (
	"context"
	"time"

	"github.com/arysis/microdash/api/internal/modules"
)

// ModulesInactifs renvoie les codes des modules qu'un compte a désactivés.
func (s *Store) ModulesInactifs(ctx context.Context, userID int64) (map[string]bool, error) {
	rows, err := s.db.Query(ctx, `SELECT code FROM modules_inactifs WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := map[string]bool{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		res[code] = true
	}
	return res, rows.Err()
}

// ActiverModule active ou désactive un module pour un compte.
func (s *Store) ActiverModule(ctx context.Context, userID int64, code string, actif bool) error {
	var err error
	if actif {
		_, err = s.db.Exec(ctx, `DELETE FROM modules_inactifs WHERE user_id = $1 AND code = $2`, userID, code)
	} else {
		_, err = s.db.Exec(ctx, `INSERT INTO modules_inactifs (user_id, code) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, code)
	}
	return err
}

// EtatsATraiter renvoie les éléments de « Ce soir » faits ou reportés, par clé source/ref.
func (s *Store) EtatsATraiter(ctx context.Context, userID int64) (map[string]modules.Etat, error) {
	rows, err := s.db.Query(ctx, `SELECT source, ref, statut, jusqua FROM a_traiter_etats WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := map[string]modules.Etat{}
	for rows.Next() {
		var source, ref, statut string
		var jusqua *time.Time
		if err := rows.Scan(&source, &ref, &statut, &jusqua); err != nil {
			return nil, err
		}
		et := modules.Etat{Statut: statut}
		if jusqua != nil {
			et.Jusqua = *jusqua
		}
		res[source+"/"+ref] = et
	}
	return res, rows.Err()
}

// MarquerATraiter note qu'un élément est fait (jusqua nul) ou reporté jusqu'à une date.
func (s *Store) MarquerATraiter(ctx context.Context, userID int64, source, ref string, jusqua *time.Time) error {
	statut := "fait"
	if jusqua != nil {
		statut = "reporte"
	}
	_, err := s.db.Exec(ctx, `INSERT INTO a_traiter_etats (user_id, source, ref, statut, jusqua) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, source, ref) DO UPDATE SET statut = $4, jusqua = $5, updated_at = now()`,
		userID, source, ref, statut, jusqua)
	return err
}
