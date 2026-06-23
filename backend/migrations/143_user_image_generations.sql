-- 用户图片生成历史与收藏

CREATE TABLE IF NOT EXISTS user_image_generations (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id       BIGINT REFERENCES api_keys(id) ON DELETE SET NULL,
    prompt           TEXT NOT NULL,
    revised_prompt   TEXT,
    model            VARCHAR(100) NOT NULL DEFAULT 'gpt-image-2',
    size             VARCHAR(32) NOT NULL DEFAULT '1024x1024',
    quality          VARCHAR(32) NOT NULL DEFAULT 'high',
    output_format    VARCHAR(32) NOT NULL DEFAULT 'webp',
    n                INT NOT NULL DEFAULT 1,
    request          JSONB NOT NULL DEFAULT '{}'::jsonb,
    reference_images JSONB NOT NULL DEFAULT '[]'::jsonb,
    images           JSONB NOT NULL DEFAULT '[]'::jsonb,
    favorite         BOOLEAN NOT NULL DEFAULT FALSE,
    status           VARCHAR(20) NOT NULL DEFAULT 'succeeded',
    error_message    TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_user_image_generations_user_created
    ON user_image_generations(user_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_image_generations_user_favorite
    ON user_image_generations(user_id, favorite, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_image_generations_api_key
    ON user_image_generations(api_key_id)
    WHERE deleted_at IS NULL;
