package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type apiKeyListFiltersCapture struct {
	service.APIKeyRepository
	filters service.APIKeyListFilters
}

func (r *apiKeyListFiltersCapture) ListByUserID(_ context.Context, _ int64, params pagination.PaginationParams, filters service.APIKeyListFilters) ([]service.APIKey, *pagination.PaginationResult, error) {
	r.filters = filters
	return []service.APIKey{}, &pagination.PaginationResult{
		Total:    0,
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    1,
	}, nil
}

func TestAPIKeyHandlerListParsesImageGenerationEnabledFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &apiKeyListFiltersCapture{}
	handler := NewAPIKeyHandler(service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil))
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})
		c.Next()
	})
	router.GET("/keys", handler.List)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/keys?status=active&image_generation_enabled=true", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, service.StatusActive, repo.filters.Status)
	require.True(t, repo.filters.ImageGenerationEnabled)
}
