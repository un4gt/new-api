package vertex

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestVertexAdaptorGetRequestURL_Embedding2EmbedContent(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		OriginModelName: "gemini-embedding-2-preview",
		RequestURLPath:  "/v1beta/models/gemini-embedding-2-preview:embedContent",
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiVersion: "global",
			ApiKey:     "test-key",
			ChannelOtherSettings: dto.ChannelOtherSettings{
				VertexKeyType: dto.VertexKeyTypeAPIKey,
			},
			UpstreamModelName: "gemini-embedding-2-preview",
		},
	}

	adaptor := &Adaptor{}
	adaptor.Init(info)

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://aiplatform.googleapis.com/v1/publishers/google/models/gemini-embedding-2-preview:embedContent?key=test-key", url)
}

func TestVertexEmbedContentHandler_PassThroughAndUsageMetadata(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-embedding-2-preview:embedContent", nil)

	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeGemini,
		OriginModelName: "gemini-embedding-2-preview",
		RequestURLPath:  "/v1beta/models/gemini-embedding-2-preview:embedContent",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-embedding-2-preview",
		},
	}
	info.SetEstimatePromptTokens(10)

	vertexRespJSON := []byte(`{"embedding":{"values":[0.1,0.2]},"usageMetadata":{"promptTokenCount":3,"totalTokenCount":3}}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(vertexRespJSON)),
	}

	usage, newAPIError := vertexEmbedContentHandler(c, resp, info)
	require.Nil(t, newAPIError)
	require.NotNil(t, usage)
	require.Equal(t, 3, usage.PromptTokens)
	require.Equal(t, 0, usage.CompletionTokens)
	require.Equal(t, 3, usage.TotalTokens)
	require.JSONEq(t, string(vertexRespJSON), w.Body.String())
}

func TestVertexAdaptorDoResponse_Embedding2EmbedContentRoutesToHandler(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-embedding-2-preview:embedContent", nil)

	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeGemini,
		OriginModelName: "gemini-embedding-2-preview",
		RequestURLPath:  "/v1beta/models/gemini-embedding-2-preview:embedContent",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-embedding-2-preview",
		},
	}

	adaptor := &Adaptor{}
	adaptor.Init(info)

	vertexRespJSON := []byte(`{"embedding":{"values":[0.1,0.2]},"usageMetadata":{"promptTokenCount":5,"totalTokenCount":5}}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(vertexRespJSON)),
	}

	usageAny, newAPIError := adaptor.DoResponse(c, resp, info)
	require.Nil(t, newAPIError)

	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 5, usage.PromptTokens)
	require.Equal(t, 5, usage.TotalTokens)
	require.JSONEq(t, string(vertexRespJSON), w.Body.String())
}
