package dto

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGeminiPartUnmarshalSnakeCaseFileData(t *testing.T) {
	raw := []byte(`{
		"file_data": {
			"mime_type": "image/jpeg",
			"file_uri": "https://example.com/a.jpg"
		}
	}`)

	var part GeminiPart
	require.NoError(t, common.Unmarshal(raw, &part))
	require.NotNil(t, part.FileData)
	require.Equal(t, "image/jpeg", part.FileData.MimeType)
	require.Equal(t, "https://example.com/a.jpg", part.FileData.FileUri)

	encoded, err := common.Marshal(part)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, common.Unmarshal(encoded, &out))

	fd, ok := out["fileData"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "image/jpeg", fd["mimeType"])
	require.Equal(t, "https://example.com/a.jpg", fd["fileUri"])
}
