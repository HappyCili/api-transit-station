package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/util/localimagestore"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDecodeImageBase64_DataURLUsesMediaType(t *testing.T) {
	data, contentType, err := decodeImageBase64("data:image/webp;base64,QUJD", "png")

	require.NoError(t, err)
	require.Equal(t, []byte("ABC"), data)
	require.Equal(t, "image/webp", contentType)
}

func TestLocalHistoryImageFileName_AcceptsAbsoluteLocalStorageURL(t *testing.T) {
	fileName, ok := localHistoryImageFileName("http://localhost:59528/storage/api-images/test-image.webp")

	require.True(t, ok)
	require.Equal(t, "test-image.webp", fileName)
}

func TestImageGenerationHandlerWriteLocalHistoryImage_ServesConfiguredStorageFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dataDir := t.TempDir()
	imageData := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
	stored, err := localimagestore.Store(dataDir, imageData, "image/png")
	require.NoError(t, err)

	handler := &ImageGenerationHandler{cfg: &config.Config{}}
	handler.cfg.Pricing.DataDir = dataDir

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/image-generations/1/images/0/view", nil)

	handled := handler.writeLocalHistoryImage(c, "http://localhost:59528"+localimagestore.URLPathPrefix+"/"+stored.FileName, "image-generation-1-1", "webp", false)

	require.True(t, handled)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
	require.Equal(t, imageData, recorder.Body.Bytes())
}

func TestRemoteImageSafetyStillRejectsLocalhost(t *testing.T) {
	require.False(t, isSafeRemoteImageHost(t.Context(), "localhost"))
	require.False(t, isSafeRemoteImageHost(t.Context(), "127.0.0.1"))
}
