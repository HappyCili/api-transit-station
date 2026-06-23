package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/internal/util/localimagestore"
	"github.com/gin-gonic/gin"
)

type ImageGenerationHandler struct {
	imageGenerationService *service.ImageGenerationService
	apiKeyService          *service.APIKeyService
	cfg                    *config.Config
}

func NewImageGenerationHandler(imageGenerationService *service.ImageGenerationService, apiKeyService *service.APIKeyService, cfg *config.Config) *ImageGenerationHandler {
	return &ImageGenerationHandler{imageGenerationService: imageGenerationService, apiKeyService: apiKeyService, cfg: cfg}
}

type createImageGenerationRequest struct {
	APIKeyID          *int64          `json:"api_key_id"`
	ConversationID    *int64          `json:"conversation_id"`
	ConversationTitle string          `json:"conversation_title"`
	Prompt            string          `json:"prompt"`
	RevisedPrompt     *string         `json:"revised_prompt"`
	Model             string          `json:"model"`
	Size              string          `json:"size"`
	Quality           string          `json:"quality"`
	OutputFormat      string          `json:"output_format"`
	N                 int             `json:"n"`
	Request           json.RawMessage `json:"request"`
	ReferenceImages   json.RawMessage `json:"reference_images"`
	Images            json.RawMessage `json:"images"`
	Status            string          `json:"status"`
	ErrorMessage      *string         `json:"error_message"`
}

type setImageGenerationFavoriteRequest struct {
	Favorite bool `json:"favorite"`
}

type imageGenerationDownloadImage struct {
	URL     string `json:"url"`
	B64JSON string `json:"b64_json"`
}

type imageGenerationResponse struct {
	ID                int64           `json:"id"`
	ConversationID    int64           `json:"conversation_id"`
	ConversationTitle string          `json:"conversation_title"`
	TurnIndex         int             `json:"turn_index"`
	UserID            int64           `json:"user_id"`
	APIKeyID          *int64          `json:"api_key_id"`
	Prompt            string          `json:"prompt"`
	RevisedPrompt     *string         `json:"revised_prompt"`
	Model             string          `json:"model"`
	Size              string          `json:"size"`
	Quality           string          `json:"quality"`
	OutputFormat      string          `json:"output_format"`
	N                 int             `json:"n"`
	Request           json.RawMessage `json:"request"`
	ReferenceImages   json.RawMessage `json:"reference_images"`
	Images            json.RawMessage `json:"images"`
	Favorite          bool            `json:"favorite"`
	Status            string          `json:"status"`
	ErrorMessage      *string         `json:"error_message"`
	CreatedAt         string          `json:"created_at"`
	UpdatedAt         string          `json:"updated_at"`
}

func (h *ImageGenerationHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    "created_at",
		SortOrder: c.DefaultQuery("sort_order", pagination.SortOrderDesc),
	}
	filters := service.ImageGenerationListFilters{
		FavoriteOnly: parseBoolQuery(c.Query("favorite")),
		Status:       strings.TrimSpace(c.Query("status")),
		Search:       strings.TrimSpace(c.Query("search")),
	}
	if conversationIDText := strings.TrimSpace(c.Query("conversation_id")); conversationIDText != "" {
		conversationID, err := strconv.ParseInt(conversationIDText, 10, 64)
		if err != nil || conversationID <= 0 {
			response.BadRequest(c, "Invalid conversation ID")
			return
		}
		filters.ConversationID = &conversationID
	}
	items, result, err := h.imageGenerationService.ListByUser(c.Request.Context(), subject.UserID, params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]imageGenerationResponse, 0, len(items))
	for i := range items {
		out = append(out, imageGenerationToResponse(&items[i]))
	}
	response.Paginated(c, out, result.Total, result.Page, result.PageSize)
}

func (h *ImageGenerationHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createImageGenerationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.APIKeyID != nil && h.apiKeyService != nil {
		key, err := h.apiKeyService.GetByID(c.Request.Context(), *req.APIKeyID)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		if key.UserID != subject.UserID {
			response.Forbidden(c, "Not authorized to use this API key")
			return
		}
	}
	item, err := h.imageGenerationService.Create(c.Request.Context(), service.CreateImageGenerationInput{
		UserID:            subject.UserID,
		ConversationID:    req.ConversationID,
		ConversationTitle: req.ConversationTitle,
		APIKeyID:          req.APIKeyID,
		Prompt:            req.Prompt,
		RevisedPrompt:     req.RevisedPrompt,
		Model:             req.Model,
		Size:              req.Size,
		Quality:           req.Quality,
		OutputFormat:      req.OutputFormat,
		N:                 req.N,
		Request:           req.Request,
		ReferenceImages:   req.ReferenceImages,
		Images:            req.Images,
		Status:            req.Status,
		ErrorMessage:      req.ErrorMessage,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, imageGenerationToResponse(item))
}

func (h *ImageGenerationHandler) DownloadImage(c *gin.Context) {
	h.writeHistoryImage(c, true)
}

func (h *ImageGenerationHandler) ViewImage(c *gin.Context) {
	h.writeHistoryImage(c, false)
}

func (h *ImageGenerationHandler) writeHistoryImage(c *gin.Context, attachment bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseImageGenerationID(c)
	if !ok {
		return
	}
	index, ok := parseImageGenerationIndex(c)
	if !ok {
		return
	}
	item, err := h.imageGenerationService.GetByUser(c.Request.Context(), subject.UserID, id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	var images []imageGenerationDownloadImage
	if err := json.Unmarshal(item.Images, &images); err != nil {
		response.BadRequest(c, "Invalid image data")
		return
	}
	if index < 0 || index >= len(images) {
		response.BadRequest(c, "Invalid image index")
		return
	}
	image := images[index]
	baseName := fmt.Sprintf("image-generation-%d-%d", id, index+1)
	if strings.TrimSpace(image.B64JSON) != "" {
		data, contentType, err := decodeImageBase64(image.B64JSON, item.OutputFormat)
		if err != nil {
			response.BadRequest(c, "Invalid image data")
			return
		}
		writeImageResponse(c, data, contentType, downloadFileName(baseName, contentType, item.OutputFormat), attachment)
		return
	}
	if strings.TrimSpace(image.URL) == "" {
		response.BadRequest(c, "Image source is empty")
		return
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(image.URL)), "data:image/") {
		data, contentType, err := decodeImageBase64(image.URL, item.OutputFormat)
		if err != nil {
			response.BadRequest(c, "Invalid image data")
			return
		}
		writeImageResponse(c, data, contentType, downloadFileName(baseName, contentType, item.OutputFormat), attachment)
		return
	}
	if h.writeLocalHistoryImage(c, image.URL, baseName, item.OutputFormat, attachment) {
		return
	}
	proxyRemoteImage(c, image.URL, baseName, item.OutputFormat, attachment)
}

func (h *ImageGenerationHandler) SetFavorite(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseImageGenerationID(c)
	if !ok {
		return
	}
	var req setImageGenerationFavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.imageGenerationService.SetFavorite(c.Request.Context(), subject.UserID, id, req.Favorite)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, imageGenerationToResponse(item))
}

func (h *ImageGenerationHandler) Delete(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	id, ok := parseImageGenerationID(c)
	if !ok {
		return
	}
	if err := h.imageGenerationService.Delete(c.Request.Context(), subject.UserID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func parseImageGenerationID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid image generation ID")
		return 0, false
	}
	return id, true
}

func parseImageGenerationIndex(c *gin.Context) (int, bool) {
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil || index < 0 {
		response.BadRequest(c, "Invalid image index")
		return 0, false
	}
	return index, true
}

func decodeImageBase64(value, outputFormat string) ([]byte, string, error) {
	contentType := imageContentTypeForFormat(outputFormat)
	base64Value := strings.TrimSpace(value)
	if strings.HasPrefix(base64Value, "data:") {
		header, data, ok := strings.Cut(base64Value, ",")
		if ok {
			base64Value = data
			if mediaType := strings.TrimPrefix(strings.Split(header, ";")[0], "data:"); strings.HasPrefix(mediaType, "image/") {
				contentType = mediaType
			}
		}
	}
	data, err := base64.StdEncoding.DecodeString(base64Value)
	if err != nil {
		return nil, "", err
	}
	return data, contentType, nil
}

func proxyRemoteImage(c *gin.Context, rawURL, baseName, fallbackFormat string, attachment bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		response.BadRequest(c, "Invalid image URL")
		return
	}
	if !isSafeRemoteImageHost(c.Request.Context(), parsed.Hostname()) {
		response.BadRequest(c, "Image URL is not allowed")
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, parsed.String(), nil)
	if err != nil {
		response.BadRequest(c, "Invalid image URL")
		return
	}
	resp, err := imageDownloadHTTPClient.Do(req)
	if err != nil {
		response.Error(c, http.StatusBadGateway, "Failed to download image")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		response.Error(c, http.StatusBadGateway, "Failed to download image")
		return
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxImageDownloadBytes+1))
	if err != nil {
		response.Error(c, http.StatusBadGateway, "Failed to download image")
		return
	}
	if int64(len(data)) > maxImageDownloadBytes {
		response.Error(c, http.StatusRequestEntityTooLarge, "Image is too large")
		return
	}
	contentType := imageMediaType(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		response.Error(c, http.StatusBadGateway, "Downloaded file is not an image")
		return
	}
	writeImageResponse(c, data, contentType, downloadFileName(baseName, contentType, fallbackFormat), attachment)
}

func (h *ImageGenerationHandler) writeLocalHistoryImage(c *gin.Context, rawURL, baseName, fallbackFormat string, attachment bool) bool {
	fileName, ok := localHistoryImageFileName(rawURL)
	if !ok {
		return false
	}
	path := filepath.Join(localimagestore.StorageDir(h.imageStorageDataDir()), fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			response.NotFound(c, "Image not found")
			return true
		}
		response.Error(c, http.StatusInternalServerError, "Failed to read image")
		return true
	}
	contentType := imageMediaType(mime.TypeByExtension(filepath.Ext(fileName)))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		response.Error(c, http.StatusBadGateway, "Stored file is not an image")
		return true
	}
	writeImageResponse(c, data, contentType, downloadFileName(baseName, contentType, fallbackFormat), attachment)
	return true
}

func (h *ImageGenerationHandler) imageStorageDataDir() string {
	if h != nil && h.cfg != nil && strings.TrimSpace(h.cfg.Pricing.DataDir) != "" {
		return h.cfg.Pricing.DataDir
	}
	return localimagestore.DefaultDataDir
}

func localHistoryImageFileName(rawURL string) (string, bool) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", false
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", false
	}
	path := parsed.Path
	if path == "" && !strings.Contains(rawURL, "://") {
		path = rawURL
	}
	prefix := localimagestore.URLPathPrefix + "/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	fileName, err := url.PathUnescape(strings.TrimPrefix(path, prefix))
	if err != nil {
		return "", false
	}
	return localimagestore.SafeFileName(fileName)
}

const maxImageDownloadBytes int64 = 50 << 20

var imageDownloadHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
	CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		if req == nil || req.URL == nil || !isSafeRemoteImageHost(req.Context(), req.URL.Hostname()) {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

func writeImageResponse(c *gin.Context, data []byte, contentType, fileName string, attachment bool) {
	if attachment {
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
		c.Header("Cache-Control", "no-store")
	} else {
		c.Header("Cache-Control", "private, max-age=300")
	}
	c.Data(http.StatusOK, contentType, data)
}

func imageContentTypeForFormat(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "jpg", "jpeg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	default:
		return "image/png"
	}
}

func imageMediaType(contentType string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return ""
	}
	return mediaType
}

func downloadFileName(baseName, contentType, fallbackFormat string) string {
	extension := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(contentType)), "image/")
	switch extension {
	case "jpeg":
		extension = "jpg"
	case "":
		extension = strings.TrimSpace(fallbackFormat)
	}
	if extension == "" {
		extension = "png"
	}
	return baseName + "." + sanitizeImageExtension(extension)
}

func sanitizeImageExtension(extension string) string {
	switch strings.ToLower(extension) {
	case "jpg", "jpeg", "png", "webp", "gif":
		return extension
	default:
		return "png"
	}
}

func isSafeRemoteImageHost(ctx context.Context, hostname string) bool {
	host := strings.ToLower(strings.TrimSpace(hostname))
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return isPublicIP(ip)
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if !isPublicIP(ip.IP) {
			return false
		}
	}
	return true
}

func isPublicIP(ip net.IP) bool {
	return ip != nil &&
		!ip.IsLoopback() &&
		!ip.IsPrivate() &&
		!ip.IsLinkLocalUnicast() &&
		!ip.IsLinkLocalMulticast() &&
		!ip.IsMulticast() &&
		!ip.IsUnspecified()
}

func imageGenerationToResponse(item *service.ImageGeneration) imageGenerationResponse {
	if item == nil {
		return imageGenerationResponse{}
	}
	return imageGenerationResponse{
		ID:                item.ID,
		ConversationID:    item.ConversationID,
		ConversationTitle: item.ConversationTitle,
		TurnIndex:         item.TurnIndex,
		UserID:            item.UserID,
		APIKeyID:          item.APIKeyID,
		Prompt:            item.Prompt,
		RevisedPrompt:     item.RevisedPrompt,
		Model:             item.Model,
		Size:              item.Size,
		Quality:           item.Quality,
		OutputFormat:      item.OutputFormat,
		N:                 item.N,
		Request:           item.Request,
		ReferenceImages:   item.ReferenceImages,
		Images:            item.Images,
		Favorite:          item.Favorite,
		Status:            item.Status,
		ErrorMessage:      item.ErrorMessage,
		CreatedAt:         item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
