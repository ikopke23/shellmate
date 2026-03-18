ALTER TABLE users ADD COLUMN IF NOT EXISTS puzzle_rating int NOT NULL DEFAULT 1500;

CREATE TABLE IF NOT EXISTS puzzles (
    id           text PRIMARY KEY,
    fen          text NOT NULL,
    moves        text NOT NULL,
    rating       int  NOT NULL,
    rating_dev   int  NOT NULL DEFAULT 0,
    popularity   int  NOT NULL DEFAULT 0,
    nb_plays     int  NOT NULL DEFAULT 0,
    themes       text[] NOT NULL DEFAULT '{}',
    game_url     text NOT NULL DEFAULT '',
    opening_tags text[] NOT NULL DEFAULT '{}',
    puzzle_date  date NOT NULL DEFAULT CURRENT_DATE,
    fetched_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_puzzle_attempts (
    id         bigserial PRIMARY KEY,
    username   text NOT NULL,
    puzzle_id  text NOT NULL REFERENCES puzzles(id),
    solved     bool NOT NULL,
    played_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_puzzle_attempts_user ON user_puzzle_attempts (username, puzzle_id);
