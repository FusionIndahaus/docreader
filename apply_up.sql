-- Extensions
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

-- Admin users
CREATE TABLE IF NOT EXISTS admin_users (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email          citext UNIQUE NOT NULL,
    password_hash  text NOT NULL,
    role           text NOT NULL DEFAULT 'admin',
    deleted_at     timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_log (
    id             bigserial PRIMARY KEY,
    actor_admin_id uuid REFERENCES admin_users(id) ON DELETE SET NULL,
    action         text NOT NULL,
    target_type    text NOT NULL,
    target_id      uuid,
    details        jsonb NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor_time ON audit_log(actor_admin_id, created_at);

-- User integrations (без foreign key на customers - используем JWT userId напрямую)
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
