package cohere

import "github.com/QuantumNous/new-api/dto"

type CohereRerankResponseResult struct {
	Results []dto.RerankResponseResult `json:"results"`
	Meta    CohereMeta                 `json:"meta"`
}

type CohereEmbedResponseResult struct {
	ID         string            `json:"id"`
	Embeddings CohereEmbeddings  `json:"embeddings"`
	Texts      []string          `json:"texts"`
	Images     []CohereImageMeta `json:"images,omitempty"`
	Meta       CohereMeta        `json:"meta"`
	Error      any               `json:"error,omitempty"`
}

type CohereEmbeddings struct {
	Float   [][]float64 `json:"float"`
	Float_  [][]float64 `json:"float_"`
	Int8    any         `json:"int8,omitempty"`
	Uint8   any         `json:"uint8,omitempty"`
	Binary  any         `json:"binary,omitempty"`
	Ubinary any         `json:"ubinary,omitempty"`
	Base64  any         `json:"base64,omitempty"`
}

type CohereImageMeta struct {
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Format   string `json:"format,omitempty"`
	BitDepth int    `json:"bit_depth,omitempty"`
}

type CohereMeta struct {
	BilledUnits CohereBilledUnits `json:"billed_units"`
	Tokens      CohereTokens      `json:"tokens"`
}

type CohereBilledUnits struct {
	InputTokens     *float64 `json:"input_tokens"`
	OutputTokens    *float64 `json:"output_tokens"`
	SearchUnits     *float64 `json:"search_units"`
	Images          *float64 `json:"images"`
	ImageTokens     *float64 `json:"image_tokens"`
	Classifications *float64 `json:"classifications"`
}

type CohereTokens struct {
	InputTokens  *float64 `json:"input_tokens"`
	OutputTokens *float64 `json:"output_tokens"`
}
