package common_handler

import (
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/xinference"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func RerankHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)
	if common.DebugEnabled {
		println("reranker response body: ", string(responseBody))
	}
	var jinaResp dto.RerankResponse
	if info.ChannelType == constant.ChannelTypeXinference {
		var xinRerankResponse xinference.XinRerankResponse
		err = common.Unmarshal(responseBody, &xinRerankResponse)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		jinaRespResults := make([]dto.RerankResponseResult, len(xinRerankResponse.Results))
		for i, result := range xinRerankResponse.Results {
			respResult := dto.RerankResponseResult{
				Index:          result.Index,
				RelevanceScore: result.RelevanceScore,
			}
			if info.ReturnDocuments {
				var document any
				if result.Document != nil {
					if doc, ok := result.Document.(string); ok {
						if doc == "" {
							document = info.Documents[result.Index]
						} else {
							document = doc
						}
					} else {
						document = result.Document
					}
				}
				respResult.Document = document
			}
			jinaRespResults[i] = respResult
		}
		jinaResp = dto.RerankResponse{
			Results: jinaRespResults,
			Usage: dto.Usage{
				PromptTokens: info.GetEstimatePromptTokens(),
				TotalTokens:  info.GetEstimatePromptTokens(),
			},
		}
	} else {
		err = common.Unmarshal(responseBody, &jinaResp)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		// Some upstreams (including OpenRouter rerank) may omit `document` fields even if the client
		// requested `return_documents`. For API compatibility, we can always reconstruct documents
		// from the original request payload by index when the upstream doesn't return them.
		if info.ReturnDocuments && len(info.Documents) > 0 {
			for i := range jinaResp.Results {
				if jinaResp.Results[i].Index < 0 || jinaResp.Results[i].Index >= len(info.Documents) {
					continue
				}
				if jinaResp.Results[i].Document == nil {
					jinaResp.Results[i].Document = info.Documents[jinaResp.Results[i].Index]
					continue
				}
				if doc, ok := jinaResp.Results[i].Document.(string); ok && doc == "" {
					jinaResp.Results[i].Document = info.Documents[jinaResp.Results[i].Index]
				}
			}
		}
		// Some upstreams (e.g. OpenRouter rerank) return cost/search_units without token usage.
		// To ensure billing works, fall back to estimated prompt tokens when token usage is absent.
		if jinaResp.Usage.TotalTokens == 0 && jinaResp.Usage.PromptTokens == 0 && jinaResp.Usage.CompletionTokens == 0 {
			estimate := info.GetEstimatePromptTokens()
			if estimate <= 0 {
				estimate = 1
			}
			jinaResp.Usage.PromptTokens = estimate
			jinaResp.Usage.TotalTokens = estimate
		} else {
			if jinaResp.Usage.TotalTokens == 0 {
				jinaResp.Usage.TotalTokens = jinaResp.Usage.PromptTokens + jinaResp.Usage.CompletionTokens
			}
			if jinaResp.Usage.PromptTokens == 0 && jinaResp.Usage.TotalTokens > 0 {
				jinaResp.Usage.PromptTokens = jinaResp.Usage.TotalTokens
			}
		}
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.JSON(http.StatusOK, jinaResp)
	return &jinaResp.Usage, nil
}
