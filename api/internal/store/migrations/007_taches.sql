-- Module Tâches du copilote. quand : la liste où la tâche est rangée (ce soir, cette semaine,
-- plus tard). source et source_ref : l'élément de « Ce soir » dont la tâche vient, s'il y en a un.
CREATE TABLE taches (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    titre         TEXT NOT NULL CHECK (length(titre) BETWEEN 1 AND 200),
    notes         TEXT NOT NULL DEFAULT '',
    projet        TEXT NOT NULL DEFAULT '' CHECK (projet IN ('', 'educari', 'saas', 'microdash', 'perso')),
    duree_minutes INT CHECK (duree_minutes BETWEEN 1 AND 1440),
    quand         TEXT NOT NULL DEFAULT 'semaine' CHECK (quand IN ('ce_soir', 'semaine', 'plus_tard')),
    echeance      DATE,
    faite_le      DATE,
    source        TEXT NOT NULL DEFAULT '',
    source_ref    TEXT NOT NULL DEFAULT '',
    lien          TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX taches_user ON taches (user_id, faite_le);
