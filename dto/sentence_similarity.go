package dto

import (
	"strings"

	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type SentenceSimilarityInputs struct {
	SourceSentence string   `json:"source_sentence"`
	Sentences      []string `json:"sentences"`
}

type SentenceSimilarityRequest struct {
	Model     string                   `json:"model"`
	Inputs    SentenceSimilarityInputs `json:"inputs"`
	Normalize *bool                    `json:"normalize,omitempty"`
}

func (r *SentenceSimilarityRequest) GetTokenCountMeta() *types.TokenCountMeta {
	texts := make([]string, 0, 1+len(r.Inputs.Sentences))
	if r.Inputs.SourceSentence != "" {
		texts = append(texts, r.Inputs.SourceSentence)
	}
	texts = append(texts, r.Inputs.Sentences...)

	return &types.TokenCountMeta{
		CombineText: strings.Join(texts, "\n"),
	}
}

func (r *SentenceSimilarityRequest) IsStream(c *gin.Context) bool {
	return false
}

func (r *SentenceSimilarityRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}
