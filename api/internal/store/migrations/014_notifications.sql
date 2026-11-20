-- Notifications Web Push (PC, téléphone, PWA installée).

-- Clé VAPID du serveur, créée au premier démarrage : les navigateurs n'acceptent que les
-- notifications signées par elle. La changer oblige chaque appareil à se réabonner.
CREATE TABLE push_cle (
    id         BOOLEAN PRIMARY KEY DEFAULT true CHECK (id),
    prive      BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Un appareil (navigateur) abonné. endpoint : adresse du service de push du navigateur.
CREATE TABLE abonnements_push (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint   TEXT NOT NULL UNIQUE,
    p256dh     TEXT NOT NULL,
    auth       TEXT NOT NULL,
    appareil   TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    erreur     TEXT NOT NULL DEFAULT ''
);
CREATE INDEX abonnements_push_user ON abonnements_push (user_id);

-- Ce que la personne veut recevoir ; sans ligne, les valeurs par défaut.
CREATE TABLE preferences_notifications (
    user_id      BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    soiree       BOOLEAN NOT NULL DEFAULT true,
    heure_soiree TIME NOT NULL DEFAULT '18:00',
    rendez_vous  BOOLEAN NOT NULL DEFAULT true,
    avance       INT NOT NULL DEFAULT 15 CHECK (avance IN (0, 5, 10, 15, 30, 60, 120)),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Une ligne par notification partie, pour ne jamais envoyer deux fois la même.
CREATE TABLE notifications_envoyees (
    user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    cle        TEXT NOT NULL,
    envoyee_le TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, cle)
);
