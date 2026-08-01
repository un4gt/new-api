package voyageai_mongodb

import (
	"errors"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type voyageRerankResponse struct {
	Data []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
		Document       *string `json:"document,omitempty"`
	} `json:"data"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func voyageRerankHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	var voyageResponse voyageRerankResponse
	if err := common.Unmarshal(responseBody, &voyageResponse); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if voyageResponse.Data == nil {
		return nil, types.NewOpenAIError(errors.New("invalid VoyageAIByMongoDB rerank response: missing data"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	results := make([]dto.RerankResponseResult, 0, len(voyageResponse.Data))
	for _, item := range voyageResponse.Data {
		result := dto.RerankResponseResult{
			Index:          item.Index,
			RelevanceScore: item.RelevanceScore,
		}
		if info != nil && info.RerankerInfo != nil && info.ReturnDocuments {
			if item.Document != nil {
				result.Document = *item.Document
			} else if item.Index >= 0 && item.Index < len(info.Documents) {
				result.Document = info.Documents[item.Index]
			}
		}
		results = append(results, result)
	}

	promptTokens := voyageResponse.Usage.TotalTokens
	if promptTokens <= 0 && info != nil {
		promptTokens = info.GetEstimatePromptTokens()
	}
	if promptTokens <= 0 {
		promptTokens = 1
	}
	usage := dto.Usage{
		PromptTokens: promptTokens,
		TotalTokens:  promptTokens,
	}
	response := dto.RerankResponse{
		Results: results,
		Usage:   usage,
	}
	jsonResponse, err := common.Marshal(response)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	service.IOCopyBytesGracefully(c, resp, jsonResponse)
	return &usage, nil
}
