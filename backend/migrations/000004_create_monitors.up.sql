CREATE TABLE monitors (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    interval_seconds INTEGER NOT NULL CHECK (interval_seconds > 0),
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX monitors_user_created_idx ON monitors (user_id, created_at DESC, id DESC);
