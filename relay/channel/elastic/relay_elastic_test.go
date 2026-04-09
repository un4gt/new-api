package elastic

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestElasticEmbeddingHandler_OpenAIEmbeddingsResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)

	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeEmbeddings,
		OriginModelName: "google-gemini-embedding-001",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "google-gemini-embedding-001",
		},
	}
	info.SetEstimatePromptTokens(10)

	elasticRespJSON := []byte(`{"text_embedding":[{"embedding":[0.1,0.2]},{"embedding":[0.3,0.4]}]}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(elasticRespJSON)),
	}

	usage, newAPIError := elasticEmbeddingHandler(c, resp, info)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 10, usage.PromptTokens)
	require.Equal(t, 10, usage.TotalTokens)

	var openAIResp dto.OpenAIEmbeddingResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &openAIResp))
	require.Equal(t, "list", openAIResp.Object)
	require.Equal(t, "google-gemini-embedding-001", openAIResp.Model)
	require.Len(t, openAIResp.Data, 2)
	require.Equal(t, 0, openAIResp.Data[0].Index)
	require.Equal(t, []float64{0.1, 0.2}, openAIResp.Data[0].Embedding)
	require.Equal(t, 1, openAIResp.Data[1].Index)
	require.Equal(t, []float64{0.3, 0.4}, openAIResp.Data[1].Embedding)
	require.Equal(t, 10, openAIResp.Usage.PromptTokens)
	require.Equal(t, 10, openAIResp.Usage.TotalTokens)
}

func TestElasticRerankHandler_ResponseConversion(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", nil)

	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeRerank,
		OriginModelName: "jina-reranker-v3",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "jina-reranker-v3",
		},
		RerankerInfo: &relaycommon.RerankerInfo{
			Documents:       []any{"doc0", "doc1"},
			ReturnDocuments: true,
		},
	}
	info.SetEstimatePromptTokens(42)

	elasticRespJSON := []byte(`{"rerank":[{"index":0,"relevance_score":1.0,"text":"ignored"},{"index":"1","relevance_score":"1.2E-2","text":"ignored"}]}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(elasticRespJSON)),
	}

	usage, newAPIError := elasticRerankHandler(c, resp, info)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 42, usage.PromptTokens)
	require.Equal(t, 42, usage.TotalTokens)

	var rerankResp dto.RerankResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &rerankResp))
	require.Len(t, rerankResp.Results, 2)
	require.Equal(t, 0, rerankResp.Results[0].Index)
	require.InDelta(t, 1.0, rerankResp.Results[0].RelevanceScore, 1e-9)
	require.Equal(t, "doc0", rerankResp.Results[0].Document)
	require.Equal(t, 1, rerankResp.Results[1].Index)
	require.InDelta(t, 0.012, rerankResp.Results[1].RelevanceScore, 1e-9)
	require.Equal(t, "doc1", rerankResp.Results[1].Document)
	require.Equal(t, 42, rerankResp.Usage.PromptTokens)
	require.Equal(t, 42, rerankResp.Usage.TotalTokens)
}
