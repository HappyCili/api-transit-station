package service

import (
	"strings"

	"github.com/tidwall/gjson"
)

// Keep the upstream Responses Lite transport contract separate from the local
// image-intent classifier so image-generation behavior remains independently owned.
const (
	responsesLiteHeader        = "X-OpenAI-Internal-Codex-Responses-Lite"
	responsesLiteHeaderKey     = "x-openai-internal-codex-responses-lite"
	responsesLiteWSMetadataKey = "ws_request_header_x_openai_internal_codex_responses_lite"
)

func isOpenAIResponsesLiteHeader(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func isOpenAIResponsesLiteWebSocketPayload(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	return isOpenAIResponsesLiteHeader(gjson.GetBytes(body, "client_metadata."+responsesLiteWSMetadataKey).String())
}
