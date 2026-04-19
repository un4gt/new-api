package xunfei_xingchen

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestXunfeiXingchenEmbeddingHandlerUsageFallback(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewBufferString(`{"id":"abc","object":"list","created":1754981944,"model":"xop3qwen0b6embedding","data":[{"index":0,"object":"embedding","embedding":[0.1,0.2]}],"usage":{}}`)),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "xc-embedding",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "sde0a5839",
		},
	}
	info.SetEstimatePromptTokens(23)

	usage, newAPIError := xunfeiEmbeddingHandler(c, resp, info)
	require.Nil(t, newAPIError)
	require.Equal(t, 23, usage.PromptTokens)
	require.Equal(t, 23, usage.TotalTokens)
	require.Contains(t, w.Body.String(), `"id":"abc"`)
	require.Contains(t, w.Body.String(), `"model":"xc-embedding"`)
	require.Contains(t, w.Body.String(), `"prompt_tokens":23`)
	require.Contains(t, w.Body.String(), `"total_tokens":23`)
}

func TestXunfeiXingchenRerankHandlerMapsModelAndReturnsDocuments(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	resp := &http.Response{
		Body: io.NopCloser(bytes.NewBufferString(`{"id":"rid","object":"list","created":1737030836,"model":"xop3qwen0b6reranker","results":[{"index":0,"relevance_score":0.2704802},{"index":2,"relevance_score":0.0011695}],"usage":{}}`)),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "xc-rerank",
		RerankerInfo: &relaycommon.RerankerInfo{
			Documents:       []any{"西瓜", "草莓", "荔枝"},
			ReturnDocuments: true,
		},
	}
	info.SetEstimatePromptTokens(131)

	usage, newAPIError := xunfeiRerankHandler(c, resp, info)
	require.Nil(t, newAPIError)
	require.Equal(t, 131, usage.PromptTokens)
	require.Equal(t, 131, usage.TotalTokens)
	require.Contains(t, w.Body.String(), `"model":"xc-rerank"`)
	require.Contains(t, w.Body.String(), `"document":"西瓜"`)
	require.Contains(t, w.Body.String(), `"document":"荔枝"`)
	require.Contains(t, w.Body.String(), `"prompt_tokens":131`)
}
