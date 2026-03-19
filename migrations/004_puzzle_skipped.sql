ALTER TABLE user_puzzle_attempts ADD COLUMN IF NOT EXISTS skipped bool NOT NULL DEFAULT false;
