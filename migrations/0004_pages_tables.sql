-- +goose Up

-- Баланс страниц пользователя по тарифу.
-- tarif_id ссылается на billing_db.tarifs (кросс-БД, без FK-ограничения).
-- По умолчанию тариф 1 (Trial) с лимитом 10 страниц.
CREATE TABLE IF NOT EXISTS pages_balance (
    id         BIGSERIAL    PRIMARY KEY,
    user_id    VARCHAR(255) NOT NULL,
    tarif_id   BIGINT       NOT NULL DEFAULT 1,
    amount     INTEGER      NOT NULL DEFAULT 10 CHECK (amount >= 0),
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_pages_balance_user UNIQUE (user_id)
);

CREATE INDEX IF NOT EXISTS idx_pages_balance_user_id  ON pages_balance (user_id);
CREATE INDEX IF NOT EXISTS idx_pages_balance_tarif_id ON pages_balance (tarif_id);

COMMENT ON TABLE  pages_balance            IS 'Текущий баланс страниц пользователя по активному тарифу';
COMMENT ON COLUMN pages_balance.user_id    IS 'ID пользователя из JWT (registration service)';
COMMENT ON COLUMN pages_balance.tarif_id   IS 'ID тарифа из billing_db.tarifs; при покупке подписки на product_id=1 обновляется';
COMMENT ON COLUMN pages_balance.amount     IS 'Остаток страниц';


-- Детализация запросов на обработку документов.
-- Одна запись = один запрос (batch). Несколько файлов одного запроса объединяются
-- по batch_id через ON CONFLICT UPSERT: doc_types накапливается, pages_count суммируется.
CREATE TABLE IF NOT EXISTS pages_details (
    id          BIGSERIAL    PRIMARY KEY,
    user_id     VARCHAR(255) NOT NULL,
    batch_id    VARCHAR(100),
    doc_types   TEXT         NOT NULL DEFAULT '',
    pages_count INTEGER      NOT NULL DEFAULT 0 CHECK (pages_count >= 0),
    status      VARCHAR(30)  NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,

    CONSTRAINT uq_pages_details_batch UNIQUE (batch_id),
    CONSTRAINT chk_pages_details_status CHECK (
        status IN ('pending', 'success', 'error', 'insufficient_pages')
    )
);

CREATE INDEX IF NOT EXISTS idx_pages_details_user_id    ON pages_details (user_id);
CREATE INDEX IF NOT EXISTS idx_pages_details_batch_id   ON pages_details (batch_id) WHERE batch_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_pages_details_status     ON pages_details (status);
CREATE INDEX IF NOT EXISTS idx_pages_details_created_at ON pages_details (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_pages_details_deleted_at ON pages_details (deleted_at) WHERE deleted_at IS NULL;

COMMENT ON TABLE  pages_details              IS 'Лог запросов на обработку документов (один запрос = одна строка per batch)';
COMMENT ON COLUMN pages_details.user_id      IS 'ID пользователя из JWT (registration service)';
COMMENT ON COLUMN pages_details.batch_id     IS 'Идентификатор группы файлов одного запроса (генерируется фронтом)';
COMMENT ON COLUMN pages_details.doc_types    IS 'Типы документов запроса через запятую: pdf,docx,jpg,png';
COMMENT ON COLUMN pages_details.pages_count  IS 'Суммарное количество страниц по всем файлам запроса';
COMMENT ON COLUMN pages_details.status       IS 'pending | success | error | insufficient_pages';
COMMENT ON COLUMN pages_details.deleted_at   IS 'Мягкое удаление';


-- +goose Down

DROP INDEX IF EXISTS idx_pages_details_deleted_at;
DROP INDEX IF EXISTS idx_pages_details_created_at;
DROP INDEX IF EXISTS idx_pages_details_status;
DROP INDEX IF EXISTS idx_pages_details_batch_id;
DROP INDEX IF EXISTS idx_pages_details_user_id;
DROP TABLE  IF EXISTS pages_details;

DROP INDEX IF EXISTS idx_pages_balance_tarif_id;
DROP INDEX IF EXISTS idx_pages_balance_user_id;
DROP TABLE  IF EXISTS pages_balance;
