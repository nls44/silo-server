-- +goose Up
-- +goose StatementBegin
INSERT INTO server_settings (key, value)
VALUES ('metadata.aggressive_auto_match', 'false')
ON CONFLICT (key) DO NOTHING;
-- +goose StatementEnd
