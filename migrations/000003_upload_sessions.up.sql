CREATE SCHEMA IF NOT EXISTS storage;

CREATE TABLE storage.upload_sessions (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    object_key TEXT NOT NULL,
    s3_upload_id TEXT NOT NULL,
    file_name TEXT NOT NULL,
    content_type TEXT NOT NULL,
    file_size BIGINT NOT NULL CHECK (file_size > 0),
    part_size BIGINT NOT NULL CHECK (part_size > 0),
    part_count INTEGER NOT NULL CHECK (part_count >= 1),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'completed', 'aborted')),
    expires_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT upload_sessions_completed_check CHECK (
        (status = 'completed' AND completed_at IS NOT NULL)
        OR (status <> 'completed' AND completed_at IS NULL)
    )
);

CREATE UNIQUE INDEX upload_sessions_object_key_unique
    ON storage.upload_sessions (object_key);

CREATE TRIGGER upload_sessions_set_updated_at
    BEFORE UPDATE ON storage.upload_sessions
    FOR EACH ROW
    EXECUTE FUNCTION platform.set_updated_at();
