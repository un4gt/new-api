package zeroentropy

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type zeroEntropyEmbedResponse struct {
	Results []struct {
		Embedding any `json:"embedding"`
	} `json:"results"`
	Usage struct {
		TotalBytes  int `json:"total_bytes"`
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func zeroEntropyEmbeddingHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var zeroResp zeroEntropyEmbedResponse
	if err := common.Unmarshal(responseBody, &zeroResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if len(zeroResp.Results) == 0 {
		return nil, types.NewOpenAIError(
			fmt.Errorf("invalid zeroentropy embedding response: missing results"),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}

	promptTokens := zeroResp.Usage.TotalTokens
	if promptTokens <= 0 {
		promptTokens = info.GetEstimatePromptTokens()
	}
	if promptTokens <= 0 {
		promptTokens = 1
	}
	usage := &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: 0,
		TotalTokens:      promptTokens,
	}

	openAIResponse := dto.FlexibleEmbeddingResponse{
		Object: "list",
		Data:   make([]dto.FlexibleEmbeddingResponseItem, 0, len(zeroResp.Results)),
		Model:  info.UpstreamModelName,
		Usage:  *usage,
	}
	for i, item := range zeroResp.Results {
		openAIResponse.Data = append(openAIResponse.Data, dto.FlexibleEmbeddingResponseItem{
			Object:    "embedding",
			Index:     i,
			Embedding: item.Embedding,
		})
	}

	out, err := common.Marshal(openAIResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, out)
	return usage, nil
}

type zeroEntropyRerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
	TotalBytes  int `json:"total_bytes"`
	TotalTokens int `json:"total_tokens"`
}

func zeroEntropyRerankHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var zeroResp zeroEntropyRerankResponse
	if err := common.Unmarshal(responseBody, &zeroResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if len(zeroResp.Results) == 0 {
		return nil, types.NewOpenAIError(
			fmt.Errorf("invalid zeroentropy rerank response: missing results"),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}

	results := make([]dto.RerankResponseResult, 0, len(zeroResp.Results))
	for _, item := range zeroResp.Results {
		result := dto.RerankResponseResult{
			Index:          item.Index,
			RelevanceScore: item.RelevanceScore,
		}
		if info.ReturnDocuments && item.Index >= 0 && item.Index < len(info.Documents) {
			result.Document = info.Documents[item.Index]
		}
		results = append(results, result)
	}

	promptTokens := zeroResp.TotalTokens
	if promptTokens <= 0 {
		promptTokens = info.GetEstimatePromptTokens()
	}
	if promptTokens <= 0 {
		promptTokens = 1
	}
	usage := &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: 0,
		TotalTokens:      promptTokens,
	}

	response := dto.RerankResponse{
		Results: results,
		Usage:   *usage,
	}
	out, err := common.Marshal(response)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, out)
	return usage, nil
}

