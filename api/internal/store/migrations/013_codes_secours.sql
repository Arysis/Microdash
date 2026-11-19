-- Codes de secours du mot de passe oublié : chaque code ne sert qu'une fois.
CREATE TABLE codes_secours_utilises (
    empreinte  TEXT PRIMARY KEY,
    user_id    BIGINT,
    utilise_le TIMESTAMPTZ NOT NULL DEFAULT now()
);
