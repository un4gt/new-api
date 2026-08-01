package dto

import (
	"encoding/json"
	"strings"

	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type EmbeddingOptions struct {
	Seed             int      `json:"seed,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"`
	TopK             int      `json:"top_k,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`
	NumPredict       int      `json:"num_predict,omitempty"`
	NumCtx           int      `json:"num_ctx,omitempty"`
}

type EmbeddingRequest struct {
	Model            string          `json:"model"`
	Input            any             `json:"input"`
	Inputs           any             `json:"inputs,omitempty"`
	InputType        *string         `json:"input_type,omitempty"`
	EncodingFormat   string          `json:"encoding_format,omitempty"`
	Dimensions       *int            `json:"dimensions,omitempty"`
	OutputDimension  *int            `json:"output_dimension,omitempty"`
	OutputDType      *string         `json:"output_dtype,omitempty"`
	OutputEncoding   *string         `json:"output_encoding,omitempty"`
	Truncation       *bool           `json:"truncation,omitempty"`
	MaxTokens        *int            `json:"max_tokens,omitempty"`
	EmbeddingTypes   []string        `json:"embedding_types,omitempty"`
	Truncate         *string         `json:"truncate,omitempty"`
	Priority         *int            `json:"priority,omitempty"`
	User             string          `json:"user,omitempty"`
	Seed             *float64        `json:"seed,omitempty"`
	Temperature      *float64        `json:"temperature,omitempty"`
	TopP             *float64        `json:"top_p,omitempty"`
	FrequencyPenalty *float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64        `json:"presence_penalty,omitempty"`
	Provider         json.RawMessage `json:"provider,omitempty"`
	// extra_body allows passing provider-specific request extensions while keeping
	// the OpenAI-style /v1/embeddings shape compatible for standard clients.
	ExtraBody json.RawMessage `json:"extra_body,omitempty"`
}

func (r *EmbeddingRequest) GetTokenCountMeta() *types.TokenCountMeta {
	texts := make([]string, 0)
	appendEmbeddingInputTexts(&texts, r.Input)
	appendEmbeddingInputTexts(&texts, r.Inputs)

	return &types.TokenCountMeta{
		CombineText: strings.Join(texts, "\n"),
	}
}

func appendEmbeddingInputTexts(texts *[]string, input any) {
	switch value := input.(type) {
	case string:
		*texts = append(*texts, value)
	case []string:
		*texts = append(*texts, value...)
	case []any:
		for _, item := range value {
			appendEmbeddingInputTexts(texts, item)
		}
	case map[string]any:
		if text, ok := value["text"].(string); ok {
			*texts = append(*texts, text)
		}
		if content, ok := value["content"]; ok {
			appendEmbeddingInputTexts(texts, content)
		}
	}
}

func (r *EmbeddingRequest) IsStream(c *gin.Context) bool {
	return false
}

func (r *EmbeddingRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}

func (r *EmbeddingRequest) ParseInput() []string {
	if r.Input == nil {
		return make([]string, 0)
	}
	var input []string
	switch r.Input.(type) {
	case string:
		input = []string{r.Input.(string)}
	case []any:
		input = make([]string, 0, len(r.Input.([]any)))
		for _, item := range r.Input.([]any) {
			if str, ok := item.(string); ok {
				input = append(input, str)
			}
		}
	}
	return input
}

type EmbeddingResponseItem struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type EmbeddingResponse struct {
	Object string                  `json:"object"`
	Data   []EmbeddingResponseItem `json:"data"`
	Model  string                  `json:"model"`
	Usage  `json:"usage"`
}

type CohereV2EmbedRequest struct {
	Model           string            `json:"model"`
	InputType       string            `json:"input_type"`
	Texts           []string          `json:"texts,omitempty"`
	Images          []string          `json:"images,omitempty"`
	Inputs          []json.RawMessage `json:"inputs,omitempty"`
	MaxTokens       *int              `json:"max_tokens,omitempty"`
	OutputDimension *int              `json:"output_dimension,omitempty"`
	EmbeddingTypes  []string          `json:"embedding_types,omitempty"`
	Truncate        *string           `json:"truncate,omitempty"`
	Priority        *int              `json:"priority,omitempty"`
}

func (r *CohereV2EmbedRequest) GetTokenCountMeta() *types.TokenCountMeta {
	var texts []string
	texts = append(texts, r.Texts...)
	for _, input := range r.Inputs {
		if len(input) > 0 {
			texts = append(texts, string(input))
		}
	}
	return &types.TokenCountMeta{
		CombineText: strings.Join(texts, "\n"),
	}
}

func (r *CohereV2EmbedRequest) IsStream(c *gin.Context) bool {
	return false
}

func (r *CohereV2EmbedRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}
