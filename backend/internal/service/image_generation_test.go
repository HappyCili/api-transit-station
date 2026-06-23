package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompactImageGenerationImages_KeepsInlineBase64ForDataURL(t *testing.T) {
	raw := json.RawMessage(`[{"url":"data:image/png;base64,QUJD","b64_json":"QUJD","base64":"QUJD","image_base64":"QUJD"}]`)

	compacted := compactImageGenerationImages(raw)

	require.True(t, json.Valid(compacted))
	require.Equal(t, "data:image/png;base64,QUJD", jsonStringAt(t, compacted, "url"))
	require.Equal(t, "QUJD", jsonStringAt(t, compacted, "b64_json"))
	require.Equal(t, "QUJD", jsonStringAt(t, compacted, "base64"))
	require.Equal(t, "QUJD", jsonStringAt(t, compacted, "image_base64"))
}

func TestCompactImageGenerationImages_StripsInlineBase64ForRemoteURL(t *testing.T) {
	raw := json.RawMessage(`[{"url":"https://example.com/image.png","b64_json":"QUJD","base64":"QUJD","image_base64":"QUJD"}]`)

	compacted := compactImageGenerationImages(raw)

	require.True(t, json.Valid(compacted))
	require.Equal(t, "https://example.com/image.png", jsonStringAt(t, compacted, "url"))
	require.Empty(t, jsonStringAt(t, compacted, "b64_json"))
	require.Empty(t, jsonStringAt(t, compacted, "base64"))
	require.Empty(t, jsonStringAt(t, compacted, "image_base64"))
}

func jsonStringAt(t *testing.T, raw json.RawMessage, key string) string {
	t.Helper()

	var images []map[string]any
	require.NoError(t, json.Unmarshal(raw, &images))
	require.Len(t, images, 1)
	value, _ := images[0][key].(string)
	return value
}
