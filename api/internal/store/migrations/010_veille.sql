-- Veille : les mails qui correspondent à un mot-clé, une adresse ou un domaine de veille sont
-- mis à part pour être résumés. Seuls ceux-là gardent le début de leur texte (4 000 caractères).
ALTER TABLE mails ADD COLUMN texte TEXT NOT NULL DEFAULT '';

CREATE TABLE mails_veille (
    id      BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- Une adresse (news@exemple.fr), un domaine (@exemple.fr) ou un mot-clé (golang).
    motif   TEXT NOT NULL CHECK (length(motif) BETWEEN 2 AND 100),
    UNIQUE (user_id, motif)
);
