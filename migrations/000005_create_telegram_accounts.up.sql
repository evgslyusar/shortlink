CREATE TABLE telegram_accounts (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    telegram_id BIGINT      NOT NULL,
    username    TEXT,
    linked_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT uq_telegram_accounts_user_id     UNIQUE (user_id),
    CONSTRAINT uq_telegram_accounts_telegram_id UNIQUE (telegram_id)
);
