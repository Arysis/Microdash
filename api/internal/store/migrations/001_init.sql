CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    token_hash BYTEA PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX sessions_user_idx ON sessions(user_id);

-- Une ligne par version du profil ; la plus récente fait foi.
CREATE TABLE profils (
    id                    BIGSERIAL PRIMARY KEY,
    user_id               BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    categorie             TEXT NOT NULL,
    categorie_secondaire  TEXT NOT NULL DEFAULT '',
    nature_cfp            TEXT NOT NULL,
    debut_activite        DATE NOT NULL,
    acre                  BOOLEAN NOT NULL DEFAULT false,
    versement_liberatoire BOOLEAN NOT NULL DEFAULT false,
    periodicite           TEXT NOT NULL CHECK (periodicite IN ('mensuelle', 'trimestrielle')),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX profils_user_idx ON profils(user_id, id DESC);

CREATE TABLE transactions (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        TEXT NOT NULL CHECK (type IN ('recette', 'depense')),
    date        DATE NOT NULL,
    centimes    BIGINT NOT NULL CHECK (centimes > 0),
    categorie   TEXT NOT NULL DEFAULT '',
    poste       TEXT NOT NULL DEFAULT '',
    libelle     TEXT NOT NULL DEFAULT '',
    tiers       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX transactions_user_date_idx ON transactions(user_id, date);
