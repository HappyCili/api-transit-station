package service

import (
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

// parseOpenAIImagesSSEUsageBytes reads completed Responses usage without
// changing the image forwarding path. tool_usage.image_gen takes precedence.
func (s *OpenAIGatewayService) parseOpenAIImagesSSEUsageBytes(payload []byte, usage *OpenAIUsage) {
	if usage == nil || !gjson.ValidBytes(payload) || gjson.GetBytes(payload, "type").String() != "response.completed" {
		return
	}
	response := gjson.GetBytes(payload, "response")
	if !response.IsObject() {
		return
	}
	if candidate, ok := parseOpenAIImageUsage(response.Get("tool_usage.image_gen")); ok {
		*usage = candidate
		return
	}
	if candidate, ok := parseOpenAIImageUsage(response.Get("usage")); ok {
		*usage = candidate
	}
}

func parseOpenAIImageUsage(value gjson.Result) (OpenAIUsage, bool) {
	if !value.IsObject() {
		return OpenAIUsage{}, false
	}
	input, inputOK := boundedJSONNonNegativeInt(value.Get("input_tokens"))
	output, outputOK := boundedJSONNonNegativeInt(value.Get("output_tokens"))
	image, imageOK := boundedJSONNonNegativeInt(value.Get("output_tokens_details.image_tokens"))
	if !inputOK || !outputOK || !imageOK {
		return OpenAIUsage{}, false
	}
	return OpenAIUsage{InputTokens: input, OutputTokens: output, ImageOutputTokens: image}, true
}

// boundedJSONNonNegativeInt accepts only finite JSON numbers representing a
// non-negative integer that fits in int. It avoids float conversion so hostile
// exponents cannot overflow or lose precision.
func boundedJSONNonNegativeInt(value gjson.Result) (int, bool) {
	if value.Type != gjson.Number {
		return 0, false
	}
	raw := strings.TrimSpace(value.Raw)
	if raw == "" || strings.HasPrefix(raw, "-") {
		return 0, false
	}

	mantissa := raw
	exponent := 0
	if marker := strings.IndexAny(mantissa, "eE"); marker >= 0 {
		exponentRaw := mantissa[marker+1:]
		mantissa = mantissa[:marker]
		parsed, ok := boundedJSONExponent(exponentRaw)
		if !ok {
			return 0, false
		}
		exponent = parsed
	}

	parts := strings.Split(mantissa, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, false
	}
	whole := parts[0]
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if !decimalDigitsOnly(whole) || (fraction != "" && !decimalDigitsOnly(fraction)) {
		return 0, false
	}

	digits := strings.TrimLeft(whole+fraction, "0")
	if digits == "" {
		return 0, true
	}
	scale := len(fraction) - exponent
	if scale > 0 {
		if scale >= len(digits) || !allZeroDigits(digits[len(digits)-scale:]) {
			return 0, false
		}
		digits = strings.TrimLeft(digits[:len(digits)-scale], "0")
		if digits == "" {
			return 0, true
		}
	} else if scale < 0 {
		if len(digits)-scale > 19 {
			return 0, false
		}
		digits += strings.Repeat("0", -scale)
	}

	parsed, err := strconv.ParseInt(digits, 10, 0)
	if err != nil {
		return 0, false
	}
	return int(parsed), true
}

func boundedJSONExponent(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	sign := 1
	if raw[0] == '+' || raw[0] == '-' {
		if raw[0] == '-' {
			sign = -1
		}
		raw = raw[1:]
	}
	if raw == "" || !decimalDigitsOnly(raw) {
		return 0, false
	}
	raw = strings.TrimLeft(raw, "0")
	if raw == "" {
		return 0, true
	}
	if len(raw) > 3 {
		return 0, false
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed > 100 {
		return 0, false
	}
	return sign * parsed, true
}

func decimalDigitsOnly(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

func allZeroDigits(value string) bool {
	for _, char := range value {
		if char != '0' {
			return false
		}
	}
	return true
}
