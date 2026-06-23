package handler

import (
	"io"
	"net/http"
	"strings"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/util/localimagestore"
	"github.com/gin-gonic/gin"
)

const openAIUploadMaxReferenceBytes = 10 << 20

// Upload handles OpenAI-compatible reference image uploads.
// POST /v1/uploads
func (h *OpenAIGatewayHandler) Upload(c *gin.Context) {
	if _, ok := middleware2.GetAPIKeyFromContext(c); !ok {
		h.errorResponse(c, http.StatusUnauthorized, "invalid_request_error", "Invalid API key")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "file is required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, openAIUploadMaxReferenceBytes+1))
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "failed to read uploaded file")
		return
	}
	if len(data) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "file is empty")
		return
	}
	if len(data) > openAIUploadMaxReferenceBytes {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "file must be 10MB or smaller")
		return
	}

	contentType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "file must be an image")
		return
	}
	purpose := strings.TrimSpace(c.PostForm("purpose"))
	if purpose == "" {
		purpose = "reference"
	}
	dataDir := localimagestore.DefaultDataDir
	frontendURL := ""
	if h.cfg != nil {
		if h.cfg.Pricing.DataDir != "" {
			dataDir = h.cfg.Pricing.DataDir
		}
		frontendURL = h.cfg.Server.FrontendURL
	}
	stored, err := localimagestore.Store(dataDir, data, contentType)
	if err != nil {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "failed to store uploaded file")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"object":       "file",
		"purpose":      purpose,
		"url":          localimagestore.PublicURL(c.Request, frontendURL, stored.FileName),
		"bytes":        stored.Bytes,
		"filename":     header.Filename,
		"content_type": stored.ContentType,
	})
}
