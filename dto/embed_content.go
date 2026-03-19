package dto

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// EmbedContentRequest matches Vertex AI publisher model embedContent request body.
// It supports both camelCase (Vertex REST) and snake_case (common in examples) field variants.
//
// https://docs.cloud.google.com/vertex-ai/generative-ai/docs/reference/rest/v1beta1/projects.locations.publishers.models/embedContent
type EmbedContentRequest struct {
	Content GeminiChatContent `json:"content"`

	// Deprecated top-level fields (kept for compatibility with docs/examples).
	Title                *string `json:"title,omitempty"`
	TaskType             *string `json:"taskType,omitempty"`
	OutputDimensionality *int    `json:"outputDimensionality,omitempty"`
	AutoTruncate         *bool   `json:"autoTruncate,omitempty"`

	// Preferred configuration container.
	EmbedContentConfig *EmbedContentConfig `json:"embedContentConfig,omitempty"`
}

// UnmarshalJSON supports snake_case and camelCase fields for EmbedContentRequest.
func (r *EmbedContentRequest) UnmarshalJSON(data []byte) error {
	type Alias EmbedContentRequest
	var aux struct {
		Alias
		TaskTypeSnake             *string `json:"task_type,omitempty"`
		OutputDimensionalitySnake *int    `json:"output_dimensionality,omitempty"`
		AutoTruncateSnake         *bool   `json:"auto_truncate,omitempty"`
		EmbedContentConfigSnake   *EmbedContentConfig `json:"embed_content_config,omitempty"`
		ConfigCompat              *EmbedContentConfig `json:"config,omitempty"`
	}

	if err := common.Unmarshal(data, &aux); err != nil {
		return err
	}

	*r = EmbedContentRequest(aux.Alias)

	if aux.TaskTypeSnake != nil {
		r.TaskType = aux.TaskTypeSnake
	}
	if aux.OutputDimensionalitySnake != nil {
		r.OutputDimensionality = aux.OutputDimensionalitySnake
	}
	if aux.AutoTruncateSnake != nil {
		r.AutoTruncate = aux.AutoTruncateSnake
	}
	if r.EmbedContentConfig == nil {
		if aux.EmbedContentConfigSnake != nil {
			r.EmbedContentConfig = aux.EmbedContentConfigSnake
		}
		if aux.ConfigCompat != nil {
			r.EmbedContentConfig = aux.ConfigCompat
		}
	}
	return nil
}

func (r *EmbedContentRequest) GetTokenCountMeta() *types.TokenCountMeta {
	var texts []string
	for _, part := range r.Content.Parts {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	return &types.TokenCountMeta{
		CombineText: strings.Join(texts, "\n"),
	}
}

func (r *EmbedContentRequest) IsStream(c *gin.Context) bool {
	return false
}

func (r *EmbedContentRequest) SetModelName(modelName string) {
	// embedContent request body does not contain model; model comes from URL.
}

type EmbedContentConfig struct {
	Title                *string `json:"title,omitempty"`
	TaskType             *string `json:"taskType,omitempty"`
	AutoTruncate         *bool   `json:"autoTruncate,omitempty"`
	OutputDimensionality *int    `json:"outputDimensionality,omitempty"`
	DocumentOcr          *bool   `json:"documentOcr,omitempty"`
	AudioTrackExtraction *bool   `json:"audioTrackExtraction,omitempty"`
}

// UnmarshalJSON supports snake_case and camelCase fields for EmbedContentConfig.
func (c *EmbedContentConfig) UnmarshalJSON(data []byte) error {
	type Alias EmbedContentConfig
	var aux struct {
		Alias
		TaskTypeSnake             *string `json:"task_type,omitempty"`
		AutoTruncateSnake         *bool   `json:"auto_truncate,omitempty"`
		OutputDimensionalitySnake *int    `json:"output_dimensionality,omitempty"`
		DocumentOcrSnake          *bool   `json:"document_ocr,omitempty"`
		AudioTrackExtractionSnake *bool   `json:"audio_track_extraction,omitempty"`
	}

	if err := common.Unmarshal(data, &aux); err != nil {
		return err
	}

	*c = EmbedContentConfig(aux.Alias)

	if aux.TaskTypeSnake != nil {
		c.TaskType = aux.TaskTypeSnake
	}
	if aux.AutoTruncateSnake != nil {
		c.AutoTruncate = aux.AutoTruncateSnake
	}
	if aux.OutputDimensionalitySnake != nil {
		c.OutputDimensionality = aux.OutputDimensionalitySnake
	}
	if aux.DocumentOcrSnake != nil {
		c.DocumentOcr = aux.DocumentOcrSnake
	}
	if aux.AudioTrackExtractionSnake != nil {
		c.AudioTrackExtraction = aux.AudioTrackExtractionSnake
	}

	return nil
}
