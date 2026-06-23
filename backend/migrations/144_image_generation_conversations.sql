-- 图片生成对话分组

ALTER TABLE user_image_generations
    ADD COLUMN IF NOT EXISTS conversation_id BIGINT,
    ADD COLUMN IF NOT EXISTS turn_index INT NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS conversation_title TEXT;

UPDATE user_image_generations
SET conversation_id = id
WHERE conversation_id IS NULL;

UPDATE user_image_generations
SET conversation_title = prompt
WHERE conversation_title IS NULL OR BTRIM(conversation_title) = '';

WITH ranked AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY user_id, conversation_id
            ORDER BY created_at ASC, id ASC
        ) AS rn
    FROM user_image_generations
    WHERE deleted_at IS NULL
)
UPDATE user_image_generations AS generation
SET turn_index = ranked.rn
FROM ranked
WHERE generation.id = ranked.id;

ALTER TABLE user_image_generations
    ALTER COLUMN conversation_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_user_image_generations_user_conversation
    ON user_image_generations(user_id, conversation_id, created_at ASC, id ASC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_image_generations_user_conversation_latest
    ON user_image_generations(user_id, conversation_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
