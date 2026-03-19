package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbedContentRequest_ConfigSnakeCasePreservesExplicitZeroValues(t *testing.T) {
	raw := []byte(`{
		"content": {"role":"user","parts":[{"text":"hello"}]},
		"output_dimensionality": 0,
		"auto_truncate": false,
		"config": {
			"task_type": "RETRIEVAL_QUERY",
			"output_dimensionality": 0,
			"auto_truncate": false,
			"document_ocr": false,
			"audio_track_extraction": false
		}
	}`)

	var req EmbedContentRequest
	require.NoError(t, common.Unmarshal(raw, &req))

	encoded, err := common.Marshal(req)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, common.Unmarshal(encoded, &out))

	// Top-level snake_case should marshal to camelCase and preserve explicit 0/false.
	assert.Contains(t, out, "outputDimensionality")
	assert.Contains(t, out, "autoTruncate")
	assert.Equal(t, float64(0), out["outputDimensionality"])
	assert.Equal(t, false, out["autoTruncate"])

	cfg, ok := out["config"].(map[string]any)
	require.True(t, ok)

	assert.Contains(t, cfg, "taskType")
	assert.Contains(t, cfg, "outputDimensionality")
	assert.Contains(t, cfg, "autoTruncate")
	assert.Contains(t, cfg, "documentOcr")
	assert.Contains(t, cfg, "audioTrackExtraction")

	assert.Equal(t, "RETRIEVAL_QUERY", cfg["taskType"])
	assert.Equal(t, float64(0), cfg["outputDimensionality"])
	assert.Equal(t, false, cfg["autoTruncate"])
	assert.Equal(t, false, cfg["documentOcr"])
	assert.Equal(t, false, cfg["audioTrackExtraction"])
}
