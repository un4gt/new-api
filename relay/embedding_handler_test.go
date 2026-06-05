package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/openrouter"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEmbeddingHelperRejectsOpenRouterGeminiEmbedding2LargeDimensions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyChannelType, constant.ChannelTypeOpenRouter)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, "https://openrouter.ai/api")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, openrouter.GeminiEmbedding2PreviewModel)
	c.Set("model_mapping", "{}")
	c.Set("status_code_mapping", "{}")
	dimensions := embedding2MaxOutputDimensionality + 1
	info := &relaycommon.RelayInfo{
		Request: &dto.EmbeddingRequest{
			Model:      openrouter.GeminiEmbedding2PreviewModel,
			Input:      "hello world",
			Dimensions: &dimensions,
		},
		OriginModelName: openrouter.GeminiEmbedding2PreviewModel,
		RelayFormat:     types.RelayFormatEmbedding,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenRouter,
			ApiType:           constant.APITypeOpenRouter,
			ChannelBaseUrl:    "https://openrouter.ai/api",
			UpstreamModelName: openrouter.GeminiEmbedding2PreviewModel,
		},
	}

	apiErr := EmbeddingHelper(c, info)

	require.NotNil(t, apiErr)
	require.Contains(t, apiErr.Error(), "dimensions must be between 1 and 3072")
}
