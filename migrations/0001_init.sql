-- +goose Up
-- Schema: base entities, soft-deletes, usage ledger, admin audit

-- Extensions
CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS citext;     -- case-insensitive email

-- Customers
CREATE TABLE IF NOT EXISTS customers (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email          citext UNIQUE NOT NULL,
    password_hash  text NOT NULL,
    name           text,
    company        text,
    status         text NOT NULL DEFAULT 'active', -- active|blocked
    metadata       jsonb NOT NULL DEFAULT '{}',
    deleted_at     timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

-- Subscriptions
CREATE TABLE IF NOT EXISTS subscriptions (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id    uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    status         text NOT NULL DEFAULT 'active',      -- active|canceled|expired
    period_start   date NOT NULL,
    period_end     date NOT NULL,
    quota_total    integer NOT NULL,
    auto_renew     boolean NOT NULL DEFAULT true,
    deleted_at     timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_customer ON subscriptions(customer_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status_end ON subscriptions(status, period_end);

-- Usage ledger (per-execution accounting)
CREATE TABLE IF NOT EXISTS usage_events (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id       uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    subscription_id   uuid NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    occurred_at       timestamptz NOT NULL DEFAULT now(),
    amount            integer NOT NULL DEFAULT 1,      -- how many units to charge (executions)
    source            text NOT NULL DEFAULT 'n8n',     -- n8n|openai|manual
    n8n_execution_id  text UNIQUE,
    input_tokens      integer,
    output_tokens     integer,
    payload           jsonb,
    note              text
);
CREATE INDEX IF NOT EXISTS idx_usage_subscription_time ON usage_events(subscription_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_usage_customer_time ON usage_events(customer_id, occurred_at);

-- Admin users (separate from customers) and audit log
CREATE TABLE IF NOT EXISTS admin_users (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email          citext UNIQUE NOT NULL,
    password_hash  text NOT NULL,
    role           text NOT NULL DEFAULT 'admin',      -- admin|support
    deleted_at     timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_log (
    id             bigserial PRIMARY KEY,
    actor_admin_id uuid REFERENCES admin_users(id) ON DELETE SET NULL,
    action         text NOT NULL,                      -- create_customer|grant_subscription|...
    target_type    text NOT NULL,                      -- customer|subscription|usage_event
    target_id      uuid,
    details        jsonb NOT NULL DEFAULT '{}',
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_actor_time ON audit_log(actor_admin_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS admin_users;
DROP INDEX IF EXISTS idx_usage_customer_time;
DROP INDEX IF EXISTS idx_usage_subscription_time;
DROP TABLE IF EXISTS usage_events;
DROP INDEX IF EXISTS idx_subscriptions_status_end;
DROP INDEX IF EXISTS idx_subscriptions_customer;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS customers;

