-- +goose Up

ALTER TABLE pages_details
    ADD COLUMN IF NOT EXISTS files_count INTEGER NOT NULL DEFAULT 0 CHECK (files_count >= 0);

COMMENT ON COLUMN pages_details.files_count IS 'Количество файлов в запросе (batch)';

-- Back-fill: each existing row is assumed to represent at least 1 file
UPDATE pages_details SET files_count = 1 WHERE files_count = 0;

-- +goose Down

ALTER TABLE pages_details DROP COLUMN IF EXISTS files_count;
