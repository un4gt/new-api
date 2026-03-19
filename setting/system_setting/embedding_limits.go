package system_setting

import "github.com/QuantumNous/new-api/setting/config"

// EmbeddingLimits contains runtime-configurable constraints for embedding endpoints.
// Values are exposed via the system options map as `embedding_limits.<field>`.
type EmbeddingLimits struct {
	// Embedding2ImageMaxMB is the maximum allowed image size (in MB) for gemini-embedding-2-preview embedContent requests.
	Embedding2ImageMaxMB int `json:"embedding2_image_max_mb"`
}

var defaultEmbeddingLimits = EmbeddingLimits{
	Embedding2ImageMaxMB: 20,
}

func init() {
	config.GlobalConfig.Register("embedding_limits", &defaultEmbeddingLimits)
}

func GetEmbeddingLimits() *EmbeddingLimits {
	return &defaultEmbeddingLimits
}
