package routes

import (
	"net/http"
	"path/filepath"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/util/localimagestore"
	"github.com/gin-gonic/gin"
)

// RegisterCommonRoutes 注册通用路由（健康检查、状态等）
func RegisterCommonRoutes(r *gin.Engine, cfg *config.Config) {
	dataDir := localimagestore.DefaultDataDir
	if cfg != nil && cfg.Pricing.DataDir != "" {
		dataDir = cfg.Pricing.DataDir
	}

	r.GET(localimagestore.URLPathPrefix+"/:filename", func(c *gin.Context) {
		fileName, ok := localimagestore.SafeFileName(c.Param("filename"))
		if !ok {
			c.Status(http.StatusNotFound)
			return
		}
		path := filepath.Join(localimagestore.StorageDir(dataDir), fileName)
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		c.File(path)
	})

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Claude Code 遥测日志（忽略，直接返回200）
	r.POST("/api/event_logging/batch", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Setup status endpoint (always returns needs_setup: false in normal mode)
	// This is used by the frontend to detect when the service has restarted after setup
	r.GET("/setup/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"needs_setup": false,
				"step":        "completed",
			},
		})
	})
}
