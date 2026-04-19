package xunfei_xingchen

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type xunfeiEmbeddingResponse struct {
	ID      string                              `json:"id,omitempty"`
	Object  string                              `json:"object,omitempty"`
	Created int64                               `json:"created,omitempty"`
	Model   string                              `json:"model,omitempty"`
	Data    []dto.FlexibleEmbeddingResponseItem `json:"data"`
	Usage   dto.Usage                           `json:"usage"`
}

type xunfeiRerankResponse struct {
	ID      string                     `json:"id,omitempty"`
	Object  string                     `json:"object,omitempty"`
	Created int64                      `json:"created,omitempty"`
	Model   string                     `json:"model,omitempty"`
	Results []dto.RerankResponseResult `json:"results"`
	Usage   dto.Usage                  `json:"usage"`
}

func responseModelName(info *relaycommon.RelayInfo) string {
	if info == nil {
		return ""
	}
	if modelName := strings.TrimSpace(info.OriginModelName); modelName != "" {
		return modelName
	}
	return strings.TrimSpace(info.UpstreamModelName)
}

func normalizeUsage(usage *dto.Usage, info *relaycommon.RelayInfo) bool {
	if usage == nil {
		return false
	}

	modified := false
	if usage.TotalTokens == 0 && usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		estimate := info.GetEstimatePromptTokens()
		if estimate <= 0 {
			estimate = 1
		}
		usage.PromptTokens = estimate
		usage.TotalTokens = estimate
		modified = true
	} else {
		if usage.TotalTokens == 0 {
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			modified = true
		}
		if usage.PromptTokens == 0 && usage.TotalTokens > 0 {
			usage.PromptTokens = usage.TotalTokens
			modified = true
		}
	}
	return modified
}

func xunfeiEmbeddingHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var embeddingResp xunfeiEmbeddingResponse
	if err := common.Unmarshal(responseBody, &embeddingResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if len(embeddingResp.Data) == 0 {
		return nil, types.NewOpenAIError(
			fmt.Errorf("invalid xunfei xingchen embedding response: missing data"),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}

	usage := embeddingResp.Usage
	normalizeUsage(&usage, info)
	embeddingResp.Usage = usage
	embeddingResp.Model = responseModelName(info)

	responseBody, err = common.Marshal(embeddingResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return &usage, nil
}

func xunfeiRerankHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var rerankResp xunfeiRerankResponse
	if err := common.Unmarshal(responseBody, &rerankResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if info.ReturnDocuments && len(info.Documents) > 0 {
		for i := range rerankResp.Results {
			index := rerankResp.Results[i].Index
			if index < 0 || index >= len(info.Documents) {
				continue
			}
			if rerankResp.Results[i].Document == nil {
				rerankResp.Results[i].Document = info.Documents[index]
				continue
			}
			if doc, ok := rerankResp.Results[i].Document.(string); ok && doc == "" {
				rerankResp.Results[i].Document = info.Documents[index]
			}
		}
	}

	usage := rerankResp.Usage
	normalizeUsage(&usage, info)
	rerankResp.Usage = usage
	rerankResp.Model = responseModelName(info)

	responseBody, err = common.Marshal(rerankResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return &usage, nil
}
