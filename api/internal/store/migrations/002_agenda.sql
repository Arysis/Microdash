-- Échéances de l'agenda cochées par la personne (code stable, ex. urssaf-2026-t3).
CREATE TABLE echeances_faites (
    user_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code     TEXT NOT NULL,
    faite_le TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, code)
);

-- Rappels par e-mail ; sans ligne, tout est activé.
CREATE TABLE preferences_alertes (
    user_id    BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    echeances  BOOLEAN NOT NULL DEFAULT true,
    plafond    BOOLEAN NOT NULL DEFAULT true,
    tva        BOOLEAN NOT NULL DEFAULT true,
    cfe        BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Une ligne par e-mail parti, pour ne jamais envoyer deux fois le même (ex. urssaf-2026-t3:j7).
CREATE TABLE alertes_envoyees (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cle        TEXT NOT NULL,
    envoyee_le TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, cle)
);
