package routes

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUserRoutesImageGenerationPathsAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	v1 := router.Group("/api/v1")

	RegisterUserRoutes(
		v1,
		&handler.Handlers{
			ImageGeneration: handler.NewImageGenerationHandler(nil, nil, nil),
		},
		servermiddleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }),
		servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }),
		nil,
	)

	registered := make(map[string]bool)
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	for _, route := range []string{
		http.MethodGet + " /api/v1/user/image-generations",
		http.MethodPost + " /api/v1/user/image-generations",
		http.MethodDelete + " /api/v1/user/image-generations",
		http.MethodPatch + " /api/v1/user/image-generations/:id/favorite",
		http.MethodDelete + " /api/v1/user/image-generations/:id",
		http.MethodGet + " /api/v1/user/image-generations/:id/images/:index/download",
		http.MethodGet + " /api/v1/user/image-generations/:id/images/:index/view",
	} {
		require.True(t, registered[route], "%s should be registered", route)
	}
}
