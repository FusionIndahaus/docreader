-- +goose Up
CREATE TABLE IF NOT EXISTS user_integrations (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    provider        text NOT NULL CHECK (provider IN ('amocrm','bitrix','1c','json')),
    enabled         boolean NOT NULL DEFAULT false,
    account_domain  text,
    credentials     jsonb NOT NULL DEFAULT '{}'::jsonb,
    config          jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, provider)
);
CREATE INDEX IF NOT EXISTS idx_user_integrations_user_provider ON user_integrations(user_id, provider);

-- +goose Down
DROP TABLE IF EXISTS user_integrations;
