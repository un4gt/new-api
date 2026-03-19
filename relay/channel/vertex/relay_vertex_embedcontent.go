package vertex

import (
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type vertexEmbedContentResponse struct {
	UsageMetadata *struct {
		PromptTokenCount int `json:"promptTokenCount"`
		TotalTokenCount  int `json:"totalTokenCount"`
	} `json:"usageMetadata,omitempty"`
}

func vertexEmbedContentHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var parsed vertexEmbedContentResponse
	if err := common.Unmarshal(responseBody, &parsed); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	usage := service.ResponseText2Usage(c, "", info.UpstreamModelName, info.GetEstimatePromptTokens())
	if parsed.UsageMetadata != nil && parsed.UsageMetadata.PromptTokenCount > 0 {
		promptTokens := parsed.UsageMetadata.PromptTokenCount
		totalTokens := parsed.UsageMetadata.TotalTokenCount
		if totalTokens <= 0 {
			totalTokens = promptTokens
		}
		completionTokens := totalTokens - promptTokens
		if completionTokens < 0 {
			completionTokens = 0
		}
		usage = &dto.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
		}
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}
