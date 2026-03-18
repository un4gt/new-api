package vertex

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

func TestVertexEmbeddingHandler_OpenAIEmbeddingsResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)

	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeEmbeddings,
		OriginModelName: "gemini-embedding-001",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-embedding-001",
		},
	}
	info.SetEstimatePromptTokens(10)

	vertexRespJSON := []byte(`{"predictions":[{"embeddings":{"values":[0.1,0.2],"statistics":{"token_count":3}}},{"embeddings":{"values":[0.3,0.4],"statistics":{"token_count":4}}}]}`)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(vertexRespJSON)),
	}

	usage, newAPIError := vertexEmbeddingHandler(c, resp, info)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 7, usage.PromptTokens)
	require.Equal(t, 0, usage.CompletionTokens)
	require.Equal(t, 7, usage.TotalTokens)

	var openAIResp dto.OpenAIEmbeddingResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &openAIResp))
	require.Equal(t, "list", openAIResp.Object)
	require.Equal(t, "gemini-embedding-001", openAIResp.Model)
	require.Len(t, openAIResp.Data, 2)
	require.Equal(t, 0, openAIResp.Data[0].Index)
	require.Equal(t, []float64{0.1, 0.2}, openAIResp.Data[0].Embedding)
	require.Equal(t, 1, openAIResp.Data[1].Index)
	require.Equal(t, []float64{0.3, 0.4}, openAIResp.Data[1].Embedding)
	require.Equal(t, 7, openAIResp.Usage.PromptTokens)
	require.Equal(t, 7, openAIResp.Usage.TotalTokens)
}

func TestVertexEmbeddingHandler_GeminiBatchEmbedContentsResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-embedding-001:batchEmbedContents", nil)

	info := &relaycommon.RelayInfo{
		RelayMode:              relayconstant.RelayModeGemini,
		IsGeminiBatchEmbedding: true,
		OriginModelName:        "gemini-embedding-001",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-embedding-001",
		},
	}
	info.SetEstimatePromptTokens(10)

	vertexRespJSON := []byte(`{"predictions":[{"embeddings":{"values":[0.1,0.2],"statistics":{"token_count":3}}},{"embeddings":{"values":[0.3,0.4],"statistics":{"token_count":4}}}]}`)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(vertexRespJSON)),
	}

	usage, newAPIError := vertexEmbeddingHandler(c, resp, info)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 7, usage.PromptTokens)

	var geminiResp dto.GeminiBatchEmbeddingResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &geminiResp))
	require.Len(t, geminiResp.Embeddings, 2)
	require.Equal(t, []float64{0.1, 0.2}, geminiResp.Embeddings[0].Values)
	require.Equal(t, []float64{0.3, 0.4}, geminiResp.Embeddings[1].Values)
}

func TestVertexEmbeddingHandler_UsageFallbackToEstimate(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)

	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeEmbeddings,
		OriginModelName: "gemini-embedding-001",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-embedding-001",
		},
	}
	info.SetEstimatePromptTokens(42)

	vertexRespJSON := []byte(`{"predictions":[{"embeddings":{"values":[0.1,0.2]}}]}`)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(vertexRespJSON)),
	}

	usage, newAPIError := vertexEmbeddingHandler(c, resp, info)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 42, usage.PromptTokens)
	require.Equal(t, 42, usage.TotalTokens)

	var openAIResp dto.OpenAIEmbeddingResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &openAIResp))
	require.Equal(t, 42, openAIResp.Usage.PromptTokens)
	require.Equal(t, 42, openAIResp.Usage.TotalTokens)
}
