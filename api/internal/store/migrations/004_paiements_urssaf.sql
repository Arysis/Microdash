-- Une dépense au poste URSSAF paie une déclaration : son code (ex. urssaf-2026-09) la rattache
-- à la période déclarée, dont elle remplace l'estimation.
ALTER TABLE transactions ADD COLUMN echeance TEXT NOT NULL DEFAULT '';
CREATE INDEX transactions_echeance ON transactions (user_id, echeance) WHERE echeance <> '';
