-- Module Agenda du copilote : rendez-vous saisis dans l'app. heure : heure de Paris ; sans
-- heure, l'événement occupe toute la journée.
CREATE TABLE evenements (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    titre         TEXT NOT NULL CHECK (length(titre) BETWEEN 1 AND 200),
    jour          DATE NOT NULL,
    heure         TIME,
    duree_minutes INT CHECK (duree_minutes BETWEEN 1 AND 1440),
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX evenements_user_jour ON evenements (user_id, jour);

-- Lien secret du flux iCal (Home Assistant, téléphone). Seule l'empreinte du jeton est gardée :
-- le lien n'est montré qu'une fois, quand il est créé.
CREATE TABLE flux_ical (
    user_id    BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    empreinte  BYTEA NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
