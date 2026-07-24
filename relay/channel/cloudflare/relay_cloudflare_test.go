package cloudflare

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCloudflareEmbeddingNativeRoundTrip(t *testing.T) {
	t.Parallel()

	var capturedPath string
	var capturedAuthorization string
	var capturedRequest CfEmbeddingRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuthorization = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(body, &capturedRequest))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"shape":[2,3],"data":[[0.1,0.2,0.3],[0.4,0.5,0.6]]},"success":true,"errors":[],"messages":[]}`))
	}))
	defer server.Close()

	gin.SetMode(gin.TestMode)
	downstreamRecorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(downstreamRecorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeEmbeddings,
		OriginModelName: "embedding-alias",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    server.URL + "/",
			ApiKey:            "account-id,api-token",
			UpstreamModelName: "@cf/baai/bge-m3",
		},
	}
	info.SetEstimatePromptTokens(17)
	adaptor := &Adaptor{}
	adaptor.Init(info)
	converted, err := adaptor.ConvertEmbeddingRequest(nil, info, dto.EmbeddingRequest{Input: []any{"first", "second"}})
	require.NoError(t, err)
	requestBody, err := common.Marshal(converted)
	require.NoError(t, err)

	responseAny, err := adaptor.DoRequest(ctx, info, bytes.NewReader(requestBody))
	require.NoError(t, err)
	response := responseAny.(*http.Response)
	require.Equal(t, http.StatusOK, response.StatusCode)
	usageAny, apiErr := adaptor.DoResponse(ctx, response, info)
	require.Nil(t, apiErr)
	usage := usageAny.(*dto.Usage)
	require.Equal(t, 17, usage.PromptTokens)
	require.Equal(t, 17, usage.TotalTokens)

	require.Equal(t, "/client/v4/accounts/account-id/ai/run/@cf/baai/bge-m3", capturedPath)
	require.Equal(t, "Bearer api-token", capturedAuthorization)
	require.Equal(t, []string{"first", "second"}, capturedRequest.Text)

	var downstream dto.OpenAIEmbeddingResponse
	require.NoError(t, common.Unmarshal(downstreamRecorder.Body.Bytes(), &downstream))
	require.Equal(t, "list", downstream.Object)
	require.Equal(t, "embedding-alias", downstream.Model)
	require.Len(t, downstream.Data, 2)
	require.Equal(t, 0, downstream.Data[0].Index)
	require.Equal(t, "embedding", downstream.Data[0].Object)
	require.Equal(t, []float64{0.1, 0.2, 0.3}, downstream.Data[0].Embedding)
	require.Equal(t, 1, downstream.Data[1].Index)
	require.Equal(t, []float64{0.4, 0.5, 0.6}, downstream.Data[1].Embedding)
	require.Equal(t, 17, downstream.Usage.PromptTokens)
	require.Equal(t, 17, downstream.Usage.TotalTokens)
}

func TestCloudflareEmbeddingBadResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		body          string
		expectedCount int
		message       string
	}{
		{name: "success false", body: `{"result":{},"success":false,"errors":[{"code":1001,"message":"invalid request"}],"messages":[]}`, message: "invalid request"},
		{name: "missing data", body: `{"result":{"shape":[1,2]},"success":true,"errors":[],"messages":[]}`, expectedCount: 1, message: "missing result.data"},
		{name: "invalid json", body: `{"result":`, message: "unexpected end"},
		{name: "shape mismatch", body: `{"result":{"shape":[1,3],"data":[[0.1,0.2]]},"success":true}`, expectedCount: 1, message: "dimension"},
		{name: "input count mismatch", body: `{"result":{"shape":[1,2],"data":[[0.1,0.2]]},"success":true}`, expectedCount: 2, message: "2 inputs"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
			response := &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(test.body)),
			}
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "@cf/baai/bge-m3"}}
			_, apiErr := cfEmbeddingHandler(ctx, info, response, test.expectedCount)
			require.NotNil(t, apiErr)
			require.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
			require.Contains(t, strings.ToLower(apiErr.Error()), strings.ToLower(test.message))
		})
	}
}

func TestCloudflareEmbeddingNon2xxResponseIsPreservedForUnifiedHandler(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"success":false,"errors":[{"message":"rate limited"}]}`))
	}))
	defer server.Close()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    server.URL,
			ApiKey:            "account,token",
			UpstreamModelName: "@cf/baai/bge-m3",
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)
	responseAny, err := adaptor.DoRequest(ctx, info, strings.NewReader(`{"text":["test"]}`))
	require.NoError(t, err)
	response := responseAny.(*http.Response)
	defer response.Body.Close()
	require.Equal(t, http.StatusTooManyRequests, response.StatusCode)
}
