CREATE TABLE IF NOT EXISTS users (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username     text UNIQUE NOT NULL,
    elo          int NOT NULL DEFAULT 1500,
    games_played int NOT NULL DEFAULT 0,
    created_at   timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS games (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    white            text REFERENCES users(username),
    black            text REFERENCES users(username),
    result           text,
    white_elo_before int,
    black_elo_before int,
    white_elo_after  int,
    black_elo_after  int,
    played_at        timestamptz DEFAULT now(),
    pgn              text
);

CREATE INDEX IF NOT EXISTS idx_games_white ON games(white);
CREATE INDEX IF NOT EXISTS idx_games_black ON games(black);
