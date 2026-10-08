-- Clé API Qonto d'un compte (identifiant + clé secrète chiffrée), lecture seule.
CREATE TABLE qonto_connexions (
    user_id      BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    identifiant  TEXT NOT NULL,
    cle          BYTEA NOT NULL,
    cle_fin      TEXT NOT NULL,
    organisation TEXT NOT NULL DEFAULT '',
    synchro_le   TIMESTAMPTZ,
    erreur       TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Chaque opération Qonto lue une fois. Elle devient une saisie (validee), attend la personne
-- (a_valider) ou est écartée (ignoree). Gardée après déconnexion pour ne jamais réimporter.
CREATE TABLE qonto_operations (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    qonto_id       TEXT NOT NULL,
    date           DATE NOT NULL,
    centimes       BIGINT NOT NULL CHECK (centimes > 0),
    type           TEXT NOT NULL CHECK (type IN ('recette', 'depense')),
    nom            TEXT NOT NULL DEFAULT '',
    reference      TEXT NOT NULL DEFAULT '',
    type_operation TEXT NOT NULL DEFAULT '',
    cle            TEXT NOT NULL DEFAULT '',
    poste_propose  TEXT NOT NULL DEFAULT '',
    echeance_proposee TEXT NOT NULL DEFAULT '',
    statut         TEXT NOT NULL CHECK (statut IN ('a_valider', 'validee', 'ignoree')),
    -- interne, regle, rapprochee, manuel : pourquoi l'opération a quitté « à valider ».
    motif          TEXT NOT NULL DEFAULT '',
    transaction_id BIGINT REFERENCES transactions(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, qonto_id)
);
CREATE INDEX qonto_operations_statut_idx ON qonto_operations (user_id, statut, date DESC);
CREATE INDEX qonto_operations_tx_idx ON qonto_operations (transaction_id) WHERE transaction_id IS NOT NULL;

-- Choix retenu pour un libellé : ignorer, ou créer la saisie avec ce poste / cette activité.
CREATE TABLE qonto_regles (
    user_id   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cle       TEXT NOT NULL,
    action    TEXT NOT NULL CHECK (action IN ('ignorer', 'valider')),
    categorie TEXT NOT NULL DEFAULT '',
    poste     TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (user_id, cle)
);
