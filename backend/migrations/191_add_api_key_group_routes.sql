-- API Key multi-group routing rules. An empty array preserves the legacy
-- single group behavior through api_keys.group_id.
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS group_routes JSONB NOT NULL DEFAULT '[]'::jsonb;
