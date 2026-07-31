package openai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModelListIncludesCurrentGPTFamilies(t *testing.T) {
	expectedModels := []string{
		"gpt-5.4",
		"gpt-5.4-2026-03-05",
		"gpt-5.4-pro",
		"gpt-5.4-pro-2026-03-05",
		"gpt-5.4-mini",
		"gpt-5.4-mini-2026-03-17",
		"gpt-5.4-nano",
		"gpt-5.4-nano-2026-03-17",
		"gpt-5.5",
		"gpt-5.5-2026-04-23",
		"gpt-5.6",
		"gpt-5.6-sol",
		"gpt-5.6-terra",
		"gpt-5.6-luna",
	}

	for _, modelName := range expectedModels {
		require.Contains(t, ModelList, modelName)
	}
}
