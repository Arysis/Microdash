-- Tâches récurrentes : un modèle qui crée tout seul une tâche d'un soir à chaque date
-- (chaque jour, chaque semaine le même jour, chaque mois le même quantième).
-- genere_jusqua : dernière date déjà créée ; une tâche supprimée à la main ne revient pas.
CREATE TABLE taches_recurrentes (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    titre         TEXT NOT NULL CHECK (length(titre) BETWEEN 1 AND 200),
    notes         TEXT NOT NULL DEFAULT '',
    projet        TEXT NOT NULL DEFAULT '' CHECK (projet IN ('', 'educari', 'saas', 'microdash', 'perso')),
    duree_minutes INT CHECK (duree_minutes BETWEEN 1 AND 1440),
    rythme        TEXT NOT NULL CHECK (rythme IN ('jour', 'semaine', 'mois')),
    depuis        DATE NOT NULL,
    genere_jusqua DATE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX taches_recurrentes_user ON taches_recurrentes (user_id);

-- Bilan de la semaine : tâches faites ou finies une semaine donnée.
CREATE INDEX taches_faite_le ON taches (user_id, faite_le) WHERE faite_le IS NOT NULL;
