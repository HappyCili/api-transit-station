package service

import (
	"strings"

	"github.com/tidwall/gjson"
)

// These helpers let the upstream Responses adapters recognize newer Codex
// declarations without replacing this workspace's image-generation flow.
func isOpenAIImageGenerationType(value string) bool {
	return strings.TrimSpace(value) == "image_generation"
}

func isOpenAIImageGenNamespaceName(value string) bool {
	return strings.TrimSpace(value) == "image_gen"
}

func openAIRequestBodyHasImageGenerationDeclaration(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	if openAIRequestBodyHasImageGenerationTool(body) || openAIInputHasImageGenerationDeclaration(gjson.GetBytes(body, "input")) {
		return true
	}
	return openAIJSONToolChoiceSelectsImageGenerationDeclaration(gjson.GetBytes(body, "tool_choice"))
}

func openAIInputHasImageGenerationDeclaration(input gjson.Result) bool {
	if !input.IsArray() {
		return false
	}
	found := false
	input.ForEach(func(_, item gjson.Result) bool {
		if openAIJSONString(item.Get("type")) != "additional_tools" {
			return true
		}
		found = openAIJSONToolsContainImageGeneration(item.Get("tools"))
		return !found
	})
	return found
}

func openAIJSONToolChoiceSelectsImageGenerationDeclaration(choice gjson.Result) bool {
	if openAIJSONToolChoiceSelectsImageGeneration(choice) {
		return true
	}
	if !choice.IsObject() {
		return false
	}
	choiceType := openAIJSONString(choice.Get("type"))
	if choiceType == "namespace" &&
		(isOpenAIImageGenNamespaceName(openAIJSONString(choice.Get("name"))) ||
			isOpenAIImageGenNamespaceName(openAIJSONString(choice.Get("namespace")))) {
		return true
	}
	if tool := choice.Get("tool"); tool.IsObject() {
		return openAIJSONToolChoiceSelectsImageGenerationDeclaration(tool)
	}
	return false
}

func openAIAnyToolChoiceSelectsImageGenerationDeclaration(choice any) bool {
	if openAIAnyToolChoiceSelectsImageGeneration(choice) {
		return true
	}
	choiceMap, ok := choice.(map[string]any)
	if !ok {
		return false
	}
	choiceType := strings.TrimSpace(firstNonEmptyString(choiceMap["type"]))
	if choiceType == "namespace" &&
		(isOpenAIImageGenNamespaceName(firstNonEmptyString(choiceMap["name"])) ||
			isOpenAIImageGenNamespaceName(firstNonEmptyString(choiceMap["namespace"]))) {
		return true
	}
	if tool, ok := choiceMap["tool"].(map[string]any); ok {
		return openAIAnyToolChoiceSelectsImageGenerationDeclaration(tool)
	}
	return false
}
