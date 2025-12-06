-- +goose Up
ALTER TABLE users ADD COLUMN theme TEXT DEFAULT 'mocha';

-- +goose Down
ALTER TABLE users DROP COLUMN theme;
