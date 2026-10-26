-- Modules du copilote qu'un compte a désactivés (un module est actif par défaut).
CREATE TABLE modules_inactifs (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code    TEXT NOT NULL,
    PRIMARY KEY (user_id, code)
);

-- Ce que la personne a décidé pour un élément de « Ce soir » : fait, ou reporté jusqu'à une date.
CREATE TABLE a_traiter_etats (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source     TEXT NOT NULL,
    ref        TEXT NOT NULL,
    statut     TEXT NOT NULL CHECK (statut IN ('fait', 'reporte')),
    jusqua     DATE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, source, ref),
    CHECK ((statut = 'reporte') = (jusqua IS NOT NULL))
);
