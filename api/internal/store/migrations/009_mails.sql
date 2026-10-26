-- Module Mails du copilote : connexion Gmail en IMAP. Le mot de passe d'application est
-- chiffré avec CLE_CHIFFREMENT.
CREATE TABLE mails_connexions (
    user_id         BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    email           TEXT NOT NULL,
    mot_de_passe    BYTEA NOT NULL,
    important_gmail BOOLEAN NOT NULL DEFAULT true,
    synchro_le      TIMESTAMPTZ,
    erreur          TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Copie des mails récents de la boîte de réception : enveloppe et extrait seulement, jamais
-- le message complet. Remplacée à chaque synchro.
CREATE TABLE mails (
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ref             TEXT NOT NULL,
    message_id      TEXT NOT NULL,
    expediteur      TEXT NOT NULL DEFAULT '',
    expediteur_nom  TEXT NOT NULL DEFAULT '',
    sujet           TEXT NOT NULL DEFAULT '',
    extrait         TEXT NOT NULL DEFAULT '',
    recu_le         TIMESTAMPTZ NOT NULL,
    lu              BOOLEAN NOT NULL DEFAULT false,
    important_gmail BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (user_id, ref)
);

-- Ce qui est important pour la personne : une adresse (paul@exemple.fr) ou un domaine (@exemple.fr).
CREATE TABLE mails_regles (
    id      BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    motif   TEXT NOT NULL CHECK (length(motif) BETWEEN 3 AND 254),
    libelle TEXT NOT NULL DEFAULT '',
    UNIQUE (user_id, motif)
);
