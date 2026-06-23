package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	ImageGenerationStatusSucceeded = "succeeded"
	ImageGenerationStatusFailed    = "failed"
)

var (
	ErrImageGenerationNotFound       = infraerrors.NotFound("IMAGE_GENERATION_NOT_FOUND", "image generation not found")
	ErrImageGenerationPromptRequired = infraerrors.BadRequest("IMAGE_GENERATION_PROMPT_REQUIRED", "prompt is required")
	ErrImageGenerationImagesRequired = infraerrors.BadRequest("IMAGE_GENERATION_IMAGES_REQUIRED", "images are required")
	ErrImageGenerationInvalidStatus  = infraerrors.BadRequest("IMAGE_GENERATION_STATUS_INVALID", "status is invalid")
)

type ImageGeneration struct {
	ID                int64
	ConversationID    int64
	ConversationTitle string
	TurnIndex         int
	UserID            int64
	APIKeyID          *int64
	Prompt            string
	RevisedPrompt     *string
	Model             string
	Size              string
	Quality           string
	OutputFormat      string
	N                 int
	Request           json.RawMessage
	ReferenceImages   json.RawMessage
	Images            json.RawMessage
	Favorite          bool
	Status            string
	ErrorMessage      *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CreateImageGenerationInput struct {
	UserID            int64
	ConversationID    *int64
	ConversationTitle string
	APIKeyID          *int64
	Prompt            string
	RevisedPrompt     *string
	Model             string
	Size              string
	Quality           string
	OutputFormat      string
	N                 int
	Request           json.RawMessage
	ReferenceImages   json.RawMessage
	Images            json.RawMessage
	Status            string
	ErrorMessage      *string
}

type ImageGenerationListFilters struct {
	FavoriteOnly   bool
	Status         string
	Search         string
	ConversationID *int64
}

type ImageGenerationRepository interface {
	Create(ctx context.Context, input CreateImageGenerationInput) (*ImageGeneration, error)
	GetByUser(ctx context.Context, userID, id int64) (*ImageGeneration, error)
	ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters ImageGenerationListFilters) ([]ImageGeneration, *pagination.PaginationResult, error)
	SetFavorite(ctx context.Context, userID, id int64, favorite bool) (*ImageGeneration, error)
	Delete(ctx context.Context, userID, id int64) error
}

type ImageGenerationService struct {
	repo ImageGenerationRepository
}

func NewImageGenerationService(repo ImageGenerationRepository) *ImageGenerationService {
	return &ImageGenerationService{repo: repo}
}

func (s *ImageGenerationService) Create(ctx context.Context, input CreateImageGenerationInput) (*ImageGeneration, error) {
	input.Prompt = strings.TrimSpace(input.Prompt)
	if input.Prompt == "" {
		return nil, ErrImageGenerationPromptRequired
	}
	input.ConversationTitle = strings.TrimSpace(input.ConversationTitle)
	if input.ConversationTitle == "" {
		input.ConversationTitle = input.Prompt
	}
	input.Model = defaultString(strings.TrimSpace(input.Model), "gpt-image-2")
	input.Size = defaultString(strings.TrimSpace(input.Size), "1024x1024")
	input.Quality = defaultString(strings.TrimSpace(input.Quality), "high")
	input.OutputFormat = defaultString(strings.TrimSpace(input.OutputFormat), "webp")
	if input.N <= 0 {
		input.N = 1
	}
	input.Status = strings.TrimSpace(input.Status)
	if input.Status == "" {
		input.Status = ImageGenerationStatusSucceeded
	}
	switch input.Status {
	case ImageGenerationStatusSucceeded:
		if len(input.Images) == 0 || string(input.Images) == "null" {
			return nil, ErrImageGenerationImagesRequired
		}
	case ImageGenerationStatusFailed:
	default:
		return nil, ErrImageGenerationInvalidStatus
	}
	input.Request = normalizeJSONRaw(input.Request, "{}")
	input.ReferenceImages = normalizeJSONRaw(input.ReferenceImages, "[]")
	input.Images = normalizeJSONRaw(input.Images, "[]")
	input.Images = compactImageGenerationImages(input.Images)
	return s.repo.Create(ctx, input)
}

func (s *ImageGenerationService) ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters ImageGenerationListFilters) ([]ImageGeneration, *pagination.PaginationResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 20
	}
	if params.SortOrder == "" {
		params.SortOrder = pagination.SortOrderDesc
	}
	filters.Status = strings.TrimSpace(filters.Status)
	filters.Search = strings.TrimSpace(filters.Search)
	return s.repo.ListByUser(ctx, userID, params, filters)
}

func (s *ImageGenerationService) GetByUser(ctx context.Context, userID, id int64) (*ImageGeneration, error) {
	if id <= 0 {
		return nil, ErrImageGenerationNotFound
	}
	return s.repo.GetByUser(ctx, userID, id)
}

func (s *ImageGenerationService) SetFavorite(ctx context.Context, userID, id int64, favorite bool) (*ImageGeneration, error) {
	return s.repo.SetFavorite(ctx, userID, id, favorite)
}

func (s *ImageGenerationService) Delete(ctx context.Context, userID, id int64) error {
	return s.repo.Delete(ctx, userID, id)
}

func normalizeJSONRaw(raw json.RawMessage, fallback string) json.RawMessage {
	if len(raw) == 0 || !json.Valid(raw) {
		return json.RawMessage(fallback)
	}
	return raw
}

func compactImageGenerationImages(raw json.RawMessage) json.RawMessage {
	var images []map[string]any
	if err := json.Unmarshal(raw, &images); err != nil {
		return raw
	}
	for _, image := range images {
		urlValue := strings.TrimSpace(firstImageString(image["url"]))
		if urlValue != "" && !isImageDataURL(urlValue) {
			delete(image, "b64_json")
			delete(image, "base64")
			delete(image, "image_base64")
		}
	}
	out, err := json.Marshal(images)
	if err != nil {
		return raw
	}
	return out
}

func firstImageString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func isImageDataURL(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "data:image/")
}

func defaultString(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
