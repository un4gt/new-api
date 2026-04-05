package moark

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMoarkAdaptorGetRequestURL(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}

	testCases := []struct {
		name      string
		relayMode int
		expected  string
		wantErr   bool
	}{
		{
			name:      "embeddings",
			relayMode: relayconstant.RelayModeEmbeddings,
			expected:  "https://ai.gitee.com/v1/embeddings",
		},
		{
			name:      "rerank",
			relayMode: relayconstant.RelayModeRerank,
			expected:  "https://ai.gitee.com/v1/rerank",
		},
		{
			name:      "sentence similarity",
			relayMode: relayconstant.RelayModeSentenceSimilarity,
			expected:  "https://ai.gitee.com/v1/sentence-similarity",
		},
		{
			name:      "rerank multimodal",
			relayMode: relayconstant.RelayModeRerankMultimodal,
			expected:  "https://ai.gitee.com/v1/rerank/multimodal",
		},
		{
			name:      "invalid mode",
			relayMode: relayconstant.RelayModeUnknown,
			wantErr:   true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			info := &relaycommon.RelayInfo{
				RelayMode: tc.relayMode,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: "https://ai.gitee.com",
				},
			}

			url, err := adaptor.GetRequestURL(info)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, url)
		})
	}
}

func TestMoarkAdaptorSetupRequestHeader(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Accept", "application/json")

	headers := http.Header{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "test-key",
		},
	}

	adaptor := &Adaptor{}
	err := adaptor.SetupRequestHeader(ctx, &headers, info)
	require.NoError(t, err)
	require.Equal(t, "Bearer test-key", headers.Get("Authorization"))
	require.Equal(t, "application/json", headers.Get("Content-Type"))
	require.Equal(t, "application/json", headers.Get("Accept"))
}

func TestMoarkAdaptorSetupRequestHeaderPassesThroughFailoverHeader(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("X-Failover-Enabled", "true")

	headers := http.Header{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "test-key",
		},
	}

	adaptor := &Adaptor{}
	err := adaptor.SetupRequestHeader(ctx, &headers, info)
	require.NoError(t, err)
	require.Equal(t, "true", headers.Get("X-Failover-Enabled"))
}

func TestMoarkAdaptorDoResponse_RoutesSentenceSimilarity(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/sentence-similarity", nil)

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeSentenceSimilarity,
	}
	info.SetEstimatePromptTokens(11)

	respBody := []byte(`[0.12,0.98]`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}

	adaptor := &Adaptor{}
	usageAny, newAPIError := adaptor.DoResponse(ctx, resp, info)
	require.Nil(t, newAPIError)

	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 11, usage.PromptTokens)
	require.Equal(t, 11, usage.TotalTokens)
	require.JSONEq(t, string(respBody), recorder.Body.String())
}

func TestMoarkAdaptorDoResponse_RoutesRerankMultimodal(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank/multimodal", nil)

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeRerankMultimodal,
	}
	info.SetEstimatePromptTokens(15)

	respBody := []byte(`[{"index":1,"score":0.76,"document":{"text":"doc"}}]`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}

	adaptor := &Adaptor{}
	usageAny, newAPIError := adaptor.DoResponse(ctx, resp, info)
	require.Nil(t, newAPIError)

	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)
	require.Equal(t, 15, usage.PromptTokens)
	require.Equal(t, 15, usage.TotalTokens)
	require.JSONEq(t, string(respBody), recorder.Body.String())
}

func TestMoarkAdaptorDoResponse_InvalidRelayMode(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeUnknown,
	}

	adaptor := &Adaptor{}
	usageAny, newAPIError := adaptor.DoResponse(ctx, nil, info)
	require.Nil(t, usageAny)
	require.NotNil(t, newAPIError)
	require.Equal(t, types.ErrorCodeInvalidRequest, newAPIError.GetErrorCode())
}
