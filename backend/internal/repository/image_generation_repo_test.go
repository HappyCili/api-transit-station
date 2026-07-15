package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestImageGenerationRepositoryCreate_NormalizesNewConversation(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &imageGenerationRepository{db: db}

	createdAt := time.Date(2026, 6, 15, 0, 28, 52, 0, time.UTC)
	updatedAt := createdAt
	requestJSON := []byte(`{"model":"gpt-image-2","prompt":"Draw a cat"}`)
	referenceJSON := []byte(`[]`)
	imagesJSON := []byte(`[{"url":"/storage/api-images/test.png"}]`)

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO user_image_generations").
		WithArgs(
			int64(7),
			sqlmock.AnyArg(),
			"Draw a cat",
			"Draw a cat",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			"gpt-image-2",
			"1024x1024",
			"high",
			"webp",
			1,
			requestJSON,
			referenceJSON,
			imagesJSON,
			service.ImageGenerationStatusSucceeded,
			sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(101)))

	mock.ExpectQuery("UPDATE user_image_generations").
		WithArgs(int64(101)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"conversation_id",
			"conversation_title",
			"turn_index",
			"user_id",
			"api_key_id",
			"prompt",
			"revised_prompt",
			"model",
			"size",
			"quality",
			"output_format",
			"n",
			"request",
			"reference_images",
			"images",
			"favorite",
			"status",
			"error_message",
			"created_at",
			"updated_at",
		}).AddRow(
			int64(101),
			int64(101),
			"Draw a cat",
			1,
			int64(7),
			nil,
			"Draw a cat",
			nil,
			"gpt-image-2",
			"1024x1024",
			"high",
			"webp",
			1,
			requestJSON,
			referenceJSON,
			imagesJSON,
			false,
			service.ImageGenerationStatusSucceeded,
			nil,
			createdAt,
			updatedAt,
		))
	mock.ExpectCommit()

	item, err := repo.Create(context.Background(), service.CreateImageGenerationInput{
		UserID:            7,
		ConversationTitle: "Draw a cat",
		Prompt:            "Draw a cat",
		Model:             "gpt-image-2",
		Size:              "1024x1024",
		Quality:           "high",
		OutputFormat:      "webp",
		N:                 1,
		Request:           requestJSON,
		ReferenceImages:   referenceJSON,
		Images:            imagesJSON,
		Status:            service.ImageGenerationStatusSucceeded,
	})
	require.NoError(t, err)
	require.NotNil(t, item)
	require.Equal(t, int64(101), item.ID)
	require.Equal(t, int64(101), item.ConversationID)
	require.Equal(t, 1, item.TurnIndex)
	require.Equal(t, "Draw a cat", item.ConversationTitle)
	require.JSONEq(t, string(imagesJSON), string(item.Images))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageGenerationRepositoryDeleteConversation_SoftDeletesEveryTurn(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &imageGenerationRepository{db: db}

	mock.ExpectExec("UPDATE user_image_generations").
		WithArgs(int64(7), int64(101)).
		WillReturnResult(sqlmock.NewResult(0, 3))

	err := repo.DeleteConversation(context.Background(), 7, 101)

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageGenerationRepositoryDeleteConversation_ReturnsNotFound(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &imageGenerationRepository{db: db}

	mock.ExpectExec("UPDATE user_image_generations").
		WithArgs(int64(7), int64(404)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.DeleteConversation(context.Background(), 7, 404)

	require.ErrorIs(t, err, service.ErrImageGenerationNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
