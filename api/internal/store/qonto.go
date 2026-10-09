package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/arysis/microdash/api/internal/calc"
)

// --- Qonto ---

type ConnexionQonto struct {
	Identifiant  string
	Cle          []byte // chiffrée
	CleFin       string
	Organisation string
	SynchroLe    *time.Time
	Erreur       string
	Depuis       time.Time
}

func (s *Store) ConnexionQonto(ctx context.Context, userID int64) (*ConnexionQonto, error) {
	c := &ConnexionQonto{}
	err := s.db.QueryRow(ctx, `SELECT identifiant, cle, cle_fin, organisation, synchro_le, erreur, created_at
		FROM qonto_connexions WHERE user_id = $1`, userID).
		Scan(&c.Identifiant, &c.Cle, &c.CleFin, &c.Organisation, &c.SynchroLe, &c.Erreur, &c.Depuis)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return c, err
}

// SaveConnexionQonto enregistre une nouvelle clé ; la prochaine synchro repart de zéro.
func (s *Store) SaveConnexionQonto(ctx context.Context, userID int64, c ConnexionQonto) error {
	_, err := s.db.Exec(ctx, `INSERT INTO qonto_connexions (user_id, identifiant, cle, cle_fin, organisation)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id) DO UPDATE SET identifiant = $2, cle = $3, cle_fin = $4, organisation = $5, synchro_le = NULL, erreur = '', created_at = now()`,
		userID, c.Identifiant, c.Cle, c.CleFin, c.Organisation)
	return err
}

// MajSynchroQonto note une synchro réussie (à l'heure donnée), ou l'erreur de la dernière tentative.
func (s *Store) MajSynchroQonto(ctx context.Context, userID int64, le time.Time, erreur string) error {
	var err error
	if erreur != "" {
		_, err = s.db.Exec(ctx, `UPDATE qonto_connexions SET erreur = $2 WHERE user_id = $1`, userID, erreur)
	} else {
		_, err = s.db.Exec(ctx, `UPDATE qonto_connexions SET synchro_le = $2, erreur = '' WHERE user_id = $1`, userID, le)
	}
	return err
}

// DeleteConnexionQonto efface la clé. Les opérations lues restent, pour ne pas les réimporter.
func (s *Store) DeleteConnexionQonto(ctx context.Context, userID int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM qonto_connexions WHERE user_id = $1`, userID)
	return err
}

// OperationQonto est une opération lue chez Qonto, sous la forme gardée par Microdash.
type OperationQonto struct {
	ID               int64     `json:"id"`
	QontoID          string    `json:"-"`
	Date             time.Time `json:"-"`
	Centimes         int64     `json:"centimes"`
	Type             string    `json:"type"`
	Nom              string    `json:"nom"`
	Reference        string    `json:"reference"`
	TypeOperation    string    `json:"type_operation"`
	Cle              string    `json:"-"`
	PostePropose     string    `json:"poste_propose"`
	EcheanceProposee string    `json:"echeance_proposee"`
	Statut           string    `json:"statut"`
	Motif            string    `json:"motif"`
	// Interne : virement entre deux comptes de la personne, jamais une recette ni une dépense.
	Interne bool `json:"-"`
}

// BilanImport compte ce qu'est devenue chaque nouvelle opération.
type BilanImport struct {
	Nouvelles   int `json:"nouvelles"`
	AValider    int `json:"a_valider"`
	Rapprochees int `json:"rapprochees"`
	Regles      int `json:"regles"`
	Ignorees    int `json:"ignorees"`
}

const fenetreRapprochement = 5 // jours

// rapprocher cherche une saisie du même type et du même montant à quelques jours près, qui
// n'est pas déjà liée à une opération Qonto : la plus proche en date.
func rapprocher(ctx context.Context, tx pgx.Tx, userID int64, o OperationQonto) (int64, bool, error) {
	var id int64
	err := tx.QueryRow(ctx, `SELECT t.id FROM transactions t
		WHERE t.user_id = $1 AND t.type = $2 AND t.centimes = $3
		  AND t.date BETWEEN $4::date - $5::int AND $4::date + $5::int
		  AND NOT EXISTS (SELECT 1 FROM qonto_operations q WHERE q.transaction_id = t.id)
		ORDER BY abs(t.date - $4::date), t.id LIMIT 1`,
		userID, o.Type, o.Centimes, o.Date, fenetreRapprochement).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	return id, err == nil, err
}

// ImporterQonto range les opérations pas encore vues : virement interne ou libellé à ignorer →
// ignorée ; saisie déjà là → rattachée ; choix retenu pour ce libellé → saisie créée ;
// sinon → à valider. categories : activités du profil, la première par défaut.
func (s *Store) ImporterQonto(ctx context.Context, userID int64, ops []OperationQonto, categories []string) (BilanImport, error) {
	var b BilanImport
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		regles := map[string]Regle{}
		rows, err := tx.Query(ctx, `SELECT cle, action, categorie, poste FROM qonto_regles WHERE user_id = $1`, userID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var r Regle
			if err := rows.Scan(&r.Cle, &r.Action, &r.Categorie, &r.Poste); err != nil {
				rows.Close()
				return err
			}
			regles[r.Cle] = r
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, o := range ops {
			var deja bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM qonto_operations WHERE user_id = $1 AND qonto_id = $2)`, userID, o.QontoID).Scan(&deja); err != nil {
				return err
			}
			if deja {
				continue
			}
			b.Nouvelles++
			o.Statut, o.Motif = "a_valider", ""
			var txID *int64
			r, aRegle := regles[o.Cle]
			switch {
			case o.Interne:
				o.Statut, o.Motif = "ignoree", "interne"
			case aRegle && r.Action == "ignorer":
				o.Statut, o.Motif = "ignoree", "regle"
			}
			if o.Statut == "a_valider" {
				id, ok, err := rapprocher(ctx, tx, userID, o)
				if err != nil {
					return err
				}
				if ok {
					o.Statut, o.Motif, txID = "validee", "rapprochee", &id
				}
			}
			if o.Statut == "a_valider" && aRegle && r.Action == "valider" {
				t, ok := r.Saisie(o, categories)
				if ok {
					var id int64
					if err := tx.QueryRow(ctx, `INSERT INTO transactions (user_id, type, date, centimes, categorie, poste, libelle, echeance)
						VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
						userID, t.Type, t.Date, t.Centimes, t.Categorie, t.Poste, t.Libelle, t.Echeance).Scan(&id); err != nil {
						return err
					}
					o.Statut, o.Motif, txID = "validee", "regle", &id
				}
			}
			if _, err := tx.Exec(ctx, `INSERT INTO qonto_operations (user_id, qonto_id, date, centimes, type, nom, reference, type_operation, cle,
				poste_propose, echeance_proposee, statut, motif, transaction_id)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
				userID, o.QontoID, o.Date, o.Centimes, o.Type, o.Nom, o.Reference, o.TypeOperation, o.Cle,
				o.PostePropose, o.EcheanceProposee, o.Statut, o.Motif, txID); err != nil {
				return err
			}
			switch {
			case o.Statut == "ignoree":
				b.Ignorees++
			case o.Motif == "rapprochee":
				b.Rapprochees++
			case o.Motif == "regle":
				b.Regles++
			default:
				b.AValider++
			}
		}
		return nil
	})
	return b, err
}

// Regle est le choix retenu pour toutes les opérations d'un même libellé.
type Regle struct {
	Cle       string
	Action    string // ignorer ou valider
	Categorie string // recette
	Poste     string // dépense
}

// Saisie donne la saisie qu'une règle « valider » crée pour une opération ; ok est faux si la
// règle ne vaut pas pour ce sens d'opération (ex. un poste de dépense pour un crédit).
func (r Regle) Saisie(o OperationQonto, categories []string) (Transaction, bool) {
	t := Transaction{Type: o.Type, Date: o.Date, Centimes: o.Centimes, Libelle: o.Nom}
	switch o.Type {
	case calc.Recette:
		if r.Categorie == "" || len(categories) == 0 {
			return t, false
		}
		t.Categorie = categories[0]
		for _, c := range categories {
			if c == r.Categorie {
				t.Categorie = c
			}
		}
	case calc.Depense:
		if r.Poste == "" {
			return t, false
		}
		t.Poste = r.Poste
		if r.Poste == calc.PosteURSSAF {
			if o.EcheanceProposee == "" {
				return t, false
			}
			t.Echeance = o.EcheanceProposee
		}
	default:
		return t, false
	}
	return t, true
}

const colonnesOp = `id, date, centimes, type, nom, reference, type_operation, poste_propose, echeance_proposee, statut, motif`

// OperationsQonto renvoie les opérations d'un statut, les plus récentes d'abord.
func (s *Store) OperationsQonto(ctx context.Context, userID int64, statut string) ([]OperationQonto, error) {
	rows, err := s.db.Query(ctx, `SELECT `+colonnesOp+` FROM qonto_operations WHERE user_id = $1 AND statut = $2
		ORDER BY date DESC, id DESC LIMIT 500`, userID, statut)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := []OperationQonto{}
	for rows.Next() {
		var o OperationQonto
		if err := rows.Scan(&o.ID, &o.Date, &o.Centimes, &o.Type, &o.Nom, &o.Reference, &o.TypeOperation,
			&o.PostePropose, &o.EcheanceProposee, &o.Statut, &o.Motif); err != nil {
			return nil, err
		}
		res = append(res, o)
	}
	return res, rows.Err()
}

// CompterAValider renvoie le nombre d'opérations qui attendent la personne.
func (s *Store) CompterAValider(ctx context.Context, userID int64) (int, error) {
	var n int
	err := s.db.QueryRow(ctx, `SELECT count(*) FROM qonto_operations WHERE user_id = $1 AND statut = 'a_valider'`, userID).Scan(&n)
	return n, err
}

// ErrDejaTraitee : l'opération n'attend plus de décision.
var ErrDejaTraitee = errors.New("opération déjà traitée")

// opAValider verrouille une opération qui doit être à valider et renvoie sa clé de libellé.
func opAValider(ctx context.Context, tx pgx.Tx, userID, id int64) (string, error) {
	var cle, statut string
	err := tx.QueryRow(ctx, `SELECT cle, statut FROM qonto_operations WHERE id = $1 AND user_id = $2 FOR UPDATE`, id, userID).Scan(&cle, &statut)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err == nil && statut != "a_valider" {
		return "", ErrDejaTraitee
	}
	return cle, err
}

func retenir(ctx context.Context, tx pgx.Tx, userID int64, r Regle) error {
	if r.Cle == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `INSERT INTO qonto_regles (user_id, cle, action, categorie, poste) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (user_id, cle) DO UPDATE SET action = $3, categorie = $4, poste = $5`,
		userID, r.Cle, r.Action, r.Categorie, r.Poste)
	return err
}

// ValiderOperationQonto crée la saisie d'une opération à valider ; avec retenu, le même choix
// s'appliquera aux opérations suivantes de même libellé.
func (s *Store) ValiderOperationQonto(ctx context.Context, userID, id int64, t Transaction, retenu bool) (*Transaction, error) {
	var res *Transaction
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		cle, err := opAValider(ctx, tx, userID, id)
		if err != nil {
			return err
		}
		res, err = scanTx(tx.QueryRow(ctx, `INSERT INTO transactions (user_id, type, date, centimes, categorie, poste, libelle, tiers, echeance)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING `+colonnesTx,
			userID, t.Type, t.Date, t.Centimes, t.Categorie, t.Poste, t.Libelle, t.Tiers, t.Echeance))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE qonto_operations SET statut = 'validee', motif = 'manuel', transaction_id = $3 WHERE id = $1 AND user_id = $2`,
			id, userID, res.ID); err != nil {
			return err
		}
		if !retenu {
			return nil
		}
		return retenir(ctx, tx, userID, Regle{Cle: cle, Action: "valider", Categorie: t.Categorie, Poste: t.Poste})
	})
	return res, err
}

// IgnorerOperationQonto écarte une opération à valider (virement perso, remboursement...).
func (s *Store) IgnorerOperationQonto(ctx context.Context, userID, id int64, retenu bool) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		cle, err := opAValider(ctx, tx, userID, id)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE qonto_operations SET statut = 'ignoree', motif = 'manuel' WHERE id = $1 AND user_id = $2`, id, userID); err != nil {
			return err
		}
		if !retenu {
			return nil
		}
		return retenir(ctx, tx, userID, Regle{Cle: cle, Action: "ignorer"})
	})
}

// RemettreOperationQonto remet une opération ignorée dans « à valider » et oublie la règle
// « ignorer » de son libellé, pour que les suivantes ne soient plus écartées.
func (s *Store) RemettreOperationQonto(ctx context.Context, userID, id int64) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var cle string
		err := tx.QueryRow(ctx, `UPDATE qonto_operations SET statut = 'a_valider', motif = ''
			WHERE id = $1 AND user_id = $2 AND statut = 'ignoree' RETURNING cle`, id, userID).Scan(&cle)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `DELETE FROM qonto_regles WHERE user_id = $1 AND cle = $2 AND action = 'ignorer'`, userID, cle)
		return err
	})
}
