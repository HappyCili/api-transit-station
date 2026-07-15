package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type imageGenerationRepository struct {
	db *sql.DB
}

func NewImageGenerationRepository(sqlDB *sql.DB) service.ImageGenerationRepository {
	return &imageGenerationRepository{db: sqlDB}
}

func (r *imageGenerationRepository) Create(ctx context.Context, input service.CreateImageGenerationInput) (*service.ImageGeneration, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	const insertQuery = `
INSERT INTO user_image_generations (
    user_id, conversation_id, conversation_title, turn_index, api_key_id, prompt, revised_prompt, model, size, quality, output_format, n,
    request, reference_images, images, status, error_message
) VALUES (
    $1,
    COALESCE($2, 0),
    COALESCE(
        (
            SELECT NULLIF(conversation_title, '')
            FROM user_image_generations
            WHERE user_id = $1 AND conversation_id = $2 AND deleted_at IS NULL
            ORDER BY created_at ASC, id ASC
            LIMIT 1
        ),
        NULLIF($3, ''),
        $4
    ),
    CASE
        WHEN $2::bigint IS NULL THEN 1
        ELSE COALESCE(
            (
                SELECT MAX(turn_index) + 1
                FROM user_image_generations
                WHERE user_id = $1 AND conversation_id = $2 AND deleted_at IS NULL
            ),
            1
        )
    END,
    $5, $4, $6, $7, $8, $9, $10, $11,
    $12::jsonb, $13::jsonb, $14::jsonb, $15, $16
)
RETURNING id`
	var id int64
	if err := tx.QueryRowContext(ctx, insertQuery,
		input.UserID,
		input.ConversationID,
		input.ConversationTitle,
		input.Prompt,
		input.APIKeyID,
		input.RevisedPrompt,
		input.Model,
		input.Size,
		input.Quality,
		input.OutputFormat,
		input.N,
		[]byte(input.Request),
		[]byte(input.ReferenceImages),
		[]byte(input.Images),
		input.Status,
		input.ErrorMessage,
	).Scan(&id); err != nil {
		return nil, err
	}

	const normalizeQuery = `
UPDATE user_image_generations
SET
    conversation_id = CASE WHEN conversation_id = 0 THEN id ELSE conversation_id END,
    conversation_title = COALESCE(NULLIF(conversation_title, ''), prompt)
WHERE id = $1
RETURNING id, conversation_id, conversation_title, turn_index, user_id, api_key_id, prompt, revised_prompt,
          model, size, quality, output_format, n, request, reference_images, images, favorite, status,
          error_message, created_at, updated_at`
	item, err := scanImageGeneration(tx.QueryRowContext(ctx, normalizeQuery, id))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (r *imageGenerationRepository) ListByUser(
	ctx context.Context,
	userID int64,
	params pagination.PaginationParams,
	filters service.ImageGenerationListFilters,
) ([]service.ImageGeneration, *pagination.PaginationResult, error) {
	where, args := buildImageGenerationWhere(userID, filters)

	var total int64
	countQuery := `SELECT COUNT(*) FROM user_image_generations ` + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, nil, err
	}

	orderBy := "created_at DESC, id DESC"
	if params.NormalizedSortOrder(pagination.SortOrderDesc) == pagination.SortOrderAsc {
		orderBy = "created_at ASC, id ASC"
	}
	query := fmt.Sprintf(`
SELECT id, conversation_id, conversation_title, turn_index, user_id, api_key_id, prompt, revised_prompt,
       model, size, quality, output_format, n, request, reference_images, images, favorite, status,
       error_message, created_at, updated_at
FROM user_image_generations
%s
ORDER BY %s
LIMIT $%d OFFSET $%d`, where, orderBy, len(args)+1, len(args)+2)
	args = append(args, params.Limit(), params.Offset())

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	items := make([]service.ImageGeneration, 0)
	for rows.Next() {
		item, err := scanImageGenerationRows(rows)
		if err != nil {
			return nil, nil, err
		}
		items = append(items, *item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *imageGenerationRepository) GetByUser(ctx context.Context, userID, id int64) (*service.ImageGeneration, error) {
	const query = `
SELECT id, conversation_id, conversation_title, turn_index, user_id, api_key_id, prompt, revised_prompt,
       model, size, quality, output_format, n, request, reference_images, images, favorite, status,
       error_message, created_at, updated_at
FROM user_image_generations
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`
	item, err := scanImageGeneration(r.db.QueryRowContext(ctx, query, id, userID))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrImageGenerationNotFound
		}
		return nil, err
	}
	return item, nil
}

func (r *imageGenerationRepository) SetFavorite(ctx context.Context, userID, id int64, favorite bool) (*service.ImageGeneration, error) {
	const query = `
UPDATE user_image_generations
SET favorite = $3, updated_at = NOW()
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL
RETURNING id, conversation_id, conversation_title, turn_index, user_id, api_key_id, prompt, revised_prompt,
          model, size, quality, output_format, n, request, reference_images, images, favorite, status,
          error_message, created_at, updated_at`
	item, err := scanImageGeneration(r.db.QueryRowContext(ctx, query, id, userID, favorite))
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrImageGenerationNotFound
		}
		return nil, err
	}
	return item, nil
}

func (r *imageGenerationRepository) Delete(ctx context.Context, userID, id int64) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE user_image_generations
SET deleted_at = NOW(), updated_at = NOW()
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`, id, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrImageGenerationNotFound
	}
	return nil
}

func (r *imageGenerationRepository) DeleteConversation(ctx context.Context, userID, conversationID int64) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE user_image_generations
SET deleted_at = NOW(), updated_at = NOW()
WHERE user_id = $1 AND conversation_id = $2 AND deleted_at IS NULL`, userID, conversationID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrImageGenerationNotFound
	}
	return nil
}

func buildImageGenerationWhere(userID int64, filters service.ImageGenerationListFilters) (string, []any) {
	clauses := []string{"user_id = $1", "deleted_at IS NULL"}
	args := []any{userID}
	if filters.FavoriteOnly {
		clauses = append(clauses, "favorite = TRUE")
	}
	if filters.Status != "" {
		args = append(args, filters.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if filters.Search != "" {
		args = append(args, "%"+filters.Search+"%")
		clauses = append(clauses, fmt.Sprintf("prompt ILIKE $%d", len(args)))
	}
	if filters.ConversationID != nil {
		args = append(args, *filters.ConversationID)
		clauses = append(clauses, fmt.Sprintf("conversation_id = $%d", len(args)))
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

type imageGenerationRowScanner interface {
	Scan(dest ...any) error
}

type rowsScanner interface {
	Scan(dest ...any) error
}

func scanImageGeneration(row imageGenerationRowScanner) (*service.ImageGeneration, error) {
	return scanImageGenerationAny(row)
}

func scanImageGenerationRows(row rowsScanner) (*service.ImageGeneration, error) {
	return scanImageGenerationAny(row)
}

func scanImageGenerationAny(row interface{ Scan(dest ...any) error }) (*service.ImageGeneration, error) {
	var item service.ImageGeneration
	var apiKeyID sql.NullInt64
	var conversationTitle sql.NullString
	var revisedPrompt sql.NullString
	var requestBytes, referenceBytes, imageBytes []byte
	var errorMessage sql.NullString
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&item.ID,
		&item.ConversationID,
		&conversationTitle,
		&item.TurnIndex,
		&item.UserID,
		&apiKeyID,
		&item.Prompt,
		&revisedPrompt,
		&item.Model,
		&item.Size,
		&item.Quality,
		&item.OutputFormat,
		&item.N,
		&requestBytes,
		&referenceBytes,
		&imageBytes,
		&item.Favorite,
		&item.Status,
		&errorMessage,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if apiKeyID.Valid {
		item.APIKeyID = &apiKeyID.Int64
	}
	if conversationTitle.Valid {
		item.ConversationTitle = conversationTitle.String
	}
	if revisedPrompt.Valid {
		item.RevisedPrompt = &revisedPrompt.String
	}
	if errorMessage.Valid {
		item.ErrorMessage = &errorMessage.String
	}
	item.Request = copyJSONRaw(requestBytes, "{}")
	item.ReferenceImages = copyJSONRaw(referenceBytes, "[]")
	item.Images = copyJSONRaw(imageBytes, "[]")
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	return &item, nil
}

func copyJSONRaw(raw []byte, fallback string) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage(fallback)
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}
