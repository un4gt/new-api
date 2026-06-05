package openai

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel/openrouter"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenRouterEmbeddingRequestURL(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode:      relayconstant.RelayModeEmbeddings,
		RequestURLPath: "/v1/embeddings",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenRouter,
			ChannelBaseUrl:    "https://openrouter.ai/api",
			UpstreamModelName: openrouter.GeminiEmbedding2PreviewModel,
		},
	}

	adaptor.Init(info)
	url, err := adaptor.GetRequestURL(info)

	require.NoError(t, err)
	require.Equal(t, "https://openrouter.ai/api/v1/embeddings", url)
}

func TestOpenRouterConvertEmbeddingRequestPassesThroughProviderFields(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenRouter,
			UpstreamModelName: openrouter.GeminiEmbedding2PreviewModel,
		},
	}
	adaptor.Init(info)

	dimensions := 768
	inputType := "query"
	req := dto.EmbeddingRequest{
		Model:          openrouter.GeminiEmbedding2PreviewModel,
		Input:          []any{"hello world"},
		InputType:      &inputType,
		Dimensions:     &dimensions,
		EncodingFormat: "float",
		Provider:       []byte(`{"order":["Google AI Studio"]}`),
		User:           "user-1",
	}

	out, err := adaptor.ConvertEmbeddingRequest(nil, info, req)

	require.NoError(t, err)
	converted, ok := out.(dto.EmbeddingRequest)
	require.True(t, ok)
	require.Equal(t, openrouter.GeminiEmbedding2PreviewModel, converted.Model)
	require.Equal(t, req.Input, converted.Input)
	require.Equal(t, req.InputType, converted.InputType)
	require.Equal(t, req.Dimensions, converted.Dimensions)
	require.JSONEq(t, `{"order":["Google AI Studio"]}`, string(converted.Provider))
	require.Equal(t, "user-1", converted.User)
}

func TestOpenRouterEmbeddingHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(bytes.NewBufferString(
			`{"object":"list","model":"google/gemini-embedding-2-preview","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":3,"total_tokens":3}}`,
		)),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: openrouter.GeminiEmbedding2PreviewModel,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenRouter,
			UpstreamModelName: openrouter.GeminiEmbedding2PreviewModel,
		},
	}

	usage, apiErr := OpenaiEmbeddingHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 3, usage.TotalTokens)
	require.JSONEq(t,
		`{"object":"list","model":"google/gemini-embedding-2-preview","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":3,"total_tokens":3}}`,
		w.Body.String(),
	)
}

func TestOpenRouterEmbeddingHandlerUsageFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(bytes.NewBufferString(
			`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}]}`,
		)),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: openrouter.GeminiEmbedding2PreviewModel,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenRouter,
			UpstreamModelName: openrouter.GeminiEmbedding2PreviewModel,
		},
	}
	info.SetEstimatePromptTokens(42)

	usage, apiErr := OpenaiEmbeddingHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.Equal(t, 42, usage.PromptTokens)
	require.Equal(t, 42, usage.TotalTokens)
	require.Contains(t, w.Body.String(), `"model":"google/gemini-embedding-2-preview"`)
	require.Contains(t, w.Body.String(), `"prompt_tokens":42`)
}

func TestOpenRouterEmbeddingHandlerEnterpriseEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(bytes.NewBufferString(
			`{"success":true,"data":{"object":"list","model":"google/gemini-embedding-2-preview","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2]}],"usage":{"prompt_tokens":5,"total_tokens":5}}}`,
		)),
	}
	enterprise := true
	info := &relaycommon.RelayInfo{
		OriginModelName: openrouter.GeminiEmbedding2PreviewModel,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenRouter,
			UpstreamModelName: openrouter.GeminiEmbedding2PreviewModel,
			ChannelOtherSettings: dto.ChannelOtherSettings{
				OpenRouterEnterprise: &enterprise,
			},
		},
	}

	usage, apiErr := OpenaiEmbeddingHandler(c, info, resp)

	require.Nil(t, apiErr)
	require.Equal(t, 5, usage.PromptTokens)
	require.Equal(t, 5, usage.TotalTokens)
	require.Contains(t, w.Body.String(), `"data":[`)
	require.NotContains(t, w.Body.String(), `"success"`)
}
