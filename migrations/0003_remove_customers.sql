-- +goose Up
-- Удаляем зависимость от таблицы customers и используем email напрямую

-- Удаляем старую таблицу user_integrations
DROP TABLE IF EXISTS user_integrations CASCADE;

-- Создаем новую таблицу user_integrations с user_id (string/varchar)
CREATE TABLE IF NOT EXISTS user_integrations (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         varchar(255) NOT NULL,
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

-- Удаляем таблицы, связанные с customers
DROP TABLE IF EXISTS usage_events CASCADE;
DROP TABLE IF EXISTS subscriptions CASCADE;
DROP TABLE IF EXISTS customers CASCADE;

-- +goose Down
-- Восстанавливаем старую структуру (если нужен откат)
DROP TABLE IF EXISTS user_integrations CASCADE;

CREATE TABLE IF NOT EXISTS customers (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email          citext UNIQUE NOT NULL,
    password_hash  text NOT NULL,
    name           text,
    company        text,
    status         text NOT NULL DEFAULT 'active',
    metadata       jsonb NOT NULL DEFAULT '{}',
    deleted_at     timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

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
