package vertex

import (
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type vertexEmbeddingPredictResponse struct {
	Predictions []struct {
		Embeddings struct {
			Statistics struct {
				TokenCount int `json:"token_count"`
			} `json:"statistics"`
			Values []float64 `json:"values"`
		} `json:"embeddings"`
	} `json:"predictions"`
}

func vertexEmbeddingHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var vertexResp vertexEmbeddingPredictResponse
	if err := common.Unmarshal(responseBody, &vertexResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	embeddings := make([][]float64, 0, len(vertexResp.Predictions))
	tokenCount := 0
	for _, prediction := range vertexResp.Predictions {
		embeddings = append(embeddings, prediction.Embeddings.Values)
		tokenCount += prediction.Embeddings.Statistics.TokenCount
	}

	var usage *dto.Usage
	if tokenCount > 0 {
		usage = &dto.Usage{
			PromptTokens:     tokenCount,
			CompletionTokens: 0,
			TotalTokens:      tokenCount,
		}
	} else {
		usage = service.ResponseText2Usage(c, "", info.UpstreamModelName, info.GetEstimatePromptTokens())
	}

	switch info.RelayMode {
	case relayconstant.RelayModeEmbeddings:
		openAIResponse := dto.OpenAIEmbeddingResponse{
			Object: "list",
			Data:   make([]dto.OpenAIEmbeddingResponseItem, 0, len(embeddings)),
			Model:  info.UpstreamModelName,
		}
		for i, emb := range embeddings {
			openAIResponse.Data = append(openAIResponse.Data, dto.OpenAIEmbeddingResponseItem{
				Object:    "embedding",
				Embedding: emb,
				Index:     i,
			})
		}
		openAIResponse.Usage = *usage

		jsonResponse, err := common.Marshal(openAIResponse)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		service.IOCopyBytesGracefully(c, resp, jsonResponse)
		return usage, nil

	case relayconstant.RelayModeGemini:
		if info.IsGeminiBatchEmbedding {
			geminiResponse := dto.GeminiBatchEmbeddingResponse{
				Embeddings: make([]*dto.ContentEmbedding, 0, len(embeddings)),
			}
			for _, emb := range embeddings {
				values := emb
				geminiResponse.Embeddings = append(geminiResponse.Embeddings, &dto.ContentEmbedding{
					Values: values,
				})
			}

			jsonResponse, err := common.Marshal(geminiResponse)
			if err != nil {
				return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
			}
			service.IOCopyBytesGracefully(c, resp, jsonResponse)
			return usage, nil
		}

		var first []float64
		if len(embeddings) > 0 {
			first = embeddings[0]
		}
		geminiResponse := dto.GeminiEmbeddingResponse{
			Embedding: dto.ContentEmbedding{
				Values: first,
			},
		}

		jsonResponse, err := common.Marshal(geminiResponse)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		service.IOCopyBytesGracefully(c, resp, jsonResponse)
		return usage, nil
	}

	// Should not happen: vertexEmbeddingHandler is only used for embeddings-related relay modes.
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}
