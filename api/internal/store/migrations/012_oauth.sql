-- Connecteur Claude : Microdash sert d'autorisation OAuth 2.1 pour son serveur MCP (/mcp).
-- Les clients s'enregistrent eux-mêmes (RFC 7591) ; seuls les retours vers Claude sont acceptés.
CREATE TABLE oauth_clients (
    id            TEXT PRIMARY KEY,
    nom           TEXT NOT NULL,
    redirect_uris TEXT[] NOT NULL,
    secret_hash   BYTEA, -- NULL : client public (PKCE seul)
    cree_le       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Un accès = un compte qui a cliqué « Autoriser » pour un client. Le supprimer coupe tous ses jetons.
CREATE TABLE oauth_acces (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    client_id  TEXT NOT NULL REFERENCES oauth_clients(id) ON DELETE CASCADE,
    cree_le    TIMESTAMPTZ NOT NULL DEFAULT now(),
    utilise_le TIMESTAMPTZ
);
CREATE INDEX oauth_acces_user_idx ON oauth_acces(user_id);

-- Codes d'autorisation : usage unique, dix minutes.
CREATE TABLE oauth_codes (
    code_hash    BYTEA PRIMARY KEY,
    acces_id     BIGINT NOT NULL REFERENCES oauth_acces(id) ON DELETE CASCADE,
    redirect_uri TEXT NOT NULL,
    challenge    TEXT NOT NULL,
    expire_le    TIMESTAMPTZ NOT NULL
);

-- Jetons d'accès (une heure) et de renouvellement (90 jours, changés à chaque usage), hachés.
CREATE TABLE oauth_jetons (
    hash      BYTEA PRIMARY KEY,
    acces_id  BIGINT NOT NULL REFERENCES oauth_acces(id) ON DELETE CASCADE,
    genre     TEXT NOT NULL CHECK (genre IN ('acces', 'renouvellement')),
    expire_le TIMESTAMPTZ NOT NULL
);
CREATE INDEX oauth_jetons_acces_idx ON oauth_jetons(acces_id);
