package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCurrentGPTFamilyDefaultRatiosMatchOfficialPricing(t *testing.T) {
	expectedModelRatios := map[string]float64{
		"gpt-5.4":                 1.25,
		"gpt-5.4-2026-03-05":      1.25,
		"gpt-5.4-pro":             15,
		"gpt-5.4-pro-2026-03-05":  15,
		"gpt-5.4-mini":            0.375,
		"gpt-5.4-mini-2026-03-17": 0.375,
		"gpt-5.4-nano":            0.1,
		"gpt-5.4-nano-2026-03-17": 0.1,
		"gpt-5.5":                 2.5,
		"gpt-5.5-2026-04-23":      2.5,
		"gpt-5.6":                 2.5,
		"gpt-5.6-sol":             2.5,
		"gpt-5.6-terra":           1.25,
		"gpt-5.6-luna":            0.5,
	}

	for modelName, expectedRatio := range expectedModelRatios {
		require.InDelta(t, expectedRatio, defaultModelRatio[modelName], 0.000001, modelName)
	}

	expectedCacheModels := []string{
		"gpt-5.4",
		"gpt-5.4-2026-03-05",
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

	for _, modelName := range expectedCacheModels {
		require.InDelta(t, 0.1, defaultCacheRatio[modelName], 0.000001, modelName)
	}

	expectedCacheWriteModels := []string{
		"gpt-5.6",
		"gpt-5.6-sol",
		"gpt-5.6-terra",
		"gpt-5.6-luna",
	}

	for _, modelName := range expectedCacheWriteModels {
		require.InDelta(t, 1.25, defaultCreateCacheRatio[modelName], 0.000001, modelName)
	}
}
