CREATE TABLE IF NOT EXISTS user_ssh_keys (
    id          bigserial PRIMARY KEY,
    username    text NOT NULL REFERENCES users(username),
    fingerprint text UNIQUE NOT NULL,
    linked_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_user_ssh_keys_username ON user_ssh_keys(username);
