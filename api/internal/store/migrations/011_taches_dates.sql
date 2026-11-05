-- Tâches : la liste « Ce soir » devient « Dates » (du … au …) ; « Semaine » garde une semaine
-- précise (du lundi au dimanche). La tâche est à faire chaque soir de sa période, et s'affiche
-- ces jours-là dans l'agenda. « Plus tard » n'a pas de dates.
ALTER TABLE taches ADD COLUMN debut DATE, ADD COLUMN fin DATE;

ALTER TABLE taches DROP CONSTRAINT taches_quand_check;
-- Jour et semaine en cours à Paris (la base est en UTC).
UPDATE taches SET quand = 'dates', debut = (now() AT TIME ZONE 'Europe/Paris')::date, fin = (now() AT TIME ZONE 'Europe/Paris')::date WHERE quand = 'ce_soir';
UPDATE taches SET debut = date_trunc('week', now() AT TIME ZONE 'Europe/Paris')::date,
    fin = date_trunc('week', now() AT TIME ZONE 'Europe/Paris')::date + 6
    WHERE quand = 'semaine';
ALTER TABLE taches ALTER COLUMN quand SET DEFAULT 'semaine';
ALTER TABLE taches ADD CONSTRAINT taches_quand_check CHECK (quand IN ('dates', 'semaine', 'plus_tard'));
ALTER TABLE taches ADD CONSTRAINT taches_periode_check CHECK (
    (quand = 'plus_tard' AND debut IS NULL AND fin IS NULL)
    OR (quand <> 'plus_tard' AND debut IS NOT NULL AND fin IS NOT NULL AND fin >= debut)
);
CREATE INDEX taches_periode ON taches (user_id, debut, fin) WHERE debut IS NOT NULL;
