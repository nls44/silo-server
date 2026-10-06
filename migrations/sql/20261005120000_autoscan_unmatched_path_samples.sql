-- +goose Up
ALTER TABLE autoscan_events
    ADD COLUMN unmatched_paths JSONB NOT NULL DEFAULT '[]'::jsonb;
