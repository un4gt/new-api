package dto

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type RerankRequest struct {
	Documents       []any  `json:"documents"`
	Query           string `json:"query"`
	Model           string `json:"model"`
	TopN            *int   `json:"top_n,omitempty"`
	TopK            *int   `json:"top_k,omitempty"`
	ReturnDocuments *bool  `json:"return_documents,omitempty"`
	Truncation      *bool  `json:"truncation,omitempty"`
	MaxChunkPerDoc  *int   `json:"max_chunk_per_doc,omitempty"`
	OverLapTokens   *int   `json:"overlap_tokens,omitempty"`

	// Provider optional fields (e.g. Cohere v2/rerank).
	MaxTokensPerDoc *int `json:"max_tokens_per_doc,omitempty"`
	Priority        *int `json:"priority,omitempty"`
}

func (r *RerankRequest) IsStream(c *gin.Context) bool {
	return false
}

func (r *RerankRequest) GetTokenCountMeta() *types.TokenCountMeta {
	var texts = make([]string, 0)

	for _, document := range r.Documents {
		texts = append(texts, fmt.Sprintf("%v", document))
	}

	if r.Query != "" {
		texts = append(texts, r.Query)
	}

	return &types.TokenCountMeta{
		CombineText: strings.Join(texts, "\n"),
	}
}

func (r *RerankRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}

func (r *RerankRequest) GetReturnDocuments() bool {
	if r.ReturnDocuments == nil {
		return false
	}
	return *r.ReturnDocuments
}

type RerankResponseResult struct {
	Document       any     `json:"document,omitempty"`
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

type RerankDocument struct {
	Text any `json:"text"`
}

type RerankResponse struct {
	Results []RerankResponseResult `json:"results"`
	Usage   Usage                  `json:"usage"`
}

type CohereV2RerankRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            *int     `json:"top_n,omitempty"`
	MaxTokensPerDoc *int     `json:"max_tokens_per_doc,omitempty"`
	Priority        *int     `json:"priority,omitempty"`
}

func (r *CohereV2RerankRequest) IsStream(c *gin.Context) bool {
	return false
}

func (r *CohereV2RerankRequest) GetTokenCountMeta() *types.TokenCountMeta {
	texts := make([]string, 0, len(r.Documents)+1)
	texts = append(texts, r.Documents...)
	if r.Query != "" {
		texts = append(texts, r.Query)
	}
	return &types.TokenCountMeta{
		CombineText: strings.Join(texts, "\n"),
	}
}

func (r *CohereV2RerankRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}
