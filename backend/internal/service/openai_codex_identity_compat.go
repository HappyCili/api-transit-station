package service

import (
	"net/http"
	"strings"
)

// ensureCodexIdentityHeaders restores the identity defaults required by the
// newer gateway paths while retaining the target's existing enforcement logic.
func ensureCodexIdentityHeaders(h http.Header) {
	if h == nil {
		return
	}
	if strings.TrimSpace(h.Get("user-agent")) == "" {
		h.Set("user-agent", codexCLIUserAgent)
	}
	if strings.TrimSpace(h.Get("originator")) == "" {
		h.Set("originator", "codex_cli_rs")
	}
	if strings.TrimSpace(h.Get("version")) == "" {
		h.Set("version", codexCLIVersion)
	}
	h.Set("OpenAI-Beta", "responses=experimental")
}
