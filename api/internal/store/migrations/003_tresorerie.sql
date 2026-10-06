-- Dépenses qui reviennent chaque mois ou chaque année (abonnements, loyer...).
-- Chaque échéance devient une saisie ; genere_jusqua évite de recréer une saisie supprimée.
CREATE TABLE depenses_recurrentes (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    libelle       TEXT NOT NULL,
    poste         TEXT NOT NULL,
    centimes      BIGINT NOT NULL CHECK (centimes > 0),
    frequence     TEXT NOT NULL CHECK (frequence IN ('mensuelle', 'annuelle')),
    debut         DATE NOT NULL,
    fin           DATE,
    genere_jusqua DATE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX depenses_recurrentes_user_idx ON depenses_recurrentes(user_id);

ALTER TABLE transactions ADD COLUMN recurrente_id BIGINT REFERENCES depenses_recurrentes(id) ON DELETE SET NULL;

-- Clé Stripe restreinte (lecture seule) d'un compte, chiffrée ; la dernière lecture est gardée en cache.
CREATE TABLE stripe_connexions (
    user_id     BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    cle         BYTEA NOT NULL,
    cle_fin     TEXT NOT NULL,
    mode        TEXT NOT NULL CHECK (mode IN ('live', 'test')),
    compte_id   TEXT NOT NULL DEFAULT '',
    compte_nom  TEXT NOT NULL DEFAULT '',
    donnees     JSONB,
    lu_le       TIMESTAMPTZ,
    erreur      TEXT NOT NULL DEFAULT '',
    -- Prévoir les recettes au MRR (true) ou à la moyenne des saisies (false).
    prevision_mrr BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Solde du compte bancaire pro saisi par la personne, point de départ de la prévision.
CREATE TABLE soldes_tresorerie (
    user_id  BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    centimes BIGINT NOT NULL,
    au       DATE NOT NULL
);
