CREATE TABLE IF NOT EXISTS uploads (
    id              VARCHAR(24)  PRIMARY KEY,
    filename        VARCHAR(255) NOT NULL,
    original_size   BIGINT       NOT NULL,
    compressed_size BIGINT       NOT NULL,
    file_hash       VARCHAR(64)  NOT NULL,
    uploaded_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ  NOT NULL,
    download_count  INTEGER      NOT NULL DEFAULT 0,
    password_hash   VARCHAR(255),
    deletion_token  VARCHAR(48)  NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_uploads_expires_at ON uploads(expires_at);
CREATE INDEX IF NOT EXISTS idx_uploads_file_hash ON uploads(file_hash);
