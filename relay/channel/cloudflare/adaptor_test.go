package cloudflare

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdaptorUsesAccountIDInURLAndTokenInHeader(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
	credential := " account-123 , secret-token "
	common.SetContextKey(ctx, appconstant.ContextKeyChannelKey, credential)
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.cloudflare.com/",
			ApiKey:            credential,
			UpstreamModelName: "@cf/baai/bge-m3",
		},
	}

	adaptor := &Adaptor{}
	adaptor.Init(info)
	require.Equal(t, "secret-token", info.ApiKey)
	require.Equal(t, credential, common.GetContextKeyString(ctx, appconstant.ContextKeyChannelKey))

	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://api.cloudflare.com/client/v4/accounts/account-123/ai/run/@cf/baai/bge-m3", requestURL)

	headers := http.Header{}
	require.NoError(t, adaptor.SetupRequestHeader(ctx, &headers, info))
	require.Equal(t, "Bearer secret-token", headers.Get("Authorization"))
	require.NotContains(t, headers.Get("Authorization"), "account-123")
	require.NotContains(t, headers.Get("Authorization"), ",")
}

func TestAdaptorMapsCloudflareAliasInEmbeddingURL(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://example.com///",
			ApiKey:            "account,token",
			UpstreamModelName: "cf/bge-large-en-v1.5",
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)
	require.Equal(t, "@cf/baai/bge-large-en-v1.5", info.UpstreamModelName)
	requestURL, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/client/v4/accounts/account/ai/run/@cf/baai/bge-large-en-v1.5", requestURL)
}

func TestAdaptorRejectsInvalidStoredCredentialAtRequestTime(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl:    "https://api.cloudflare.com",
			ApiKey:            "legacy-token-only",
			UpstreamModelName: "@cf/baai/bge-m3",
		},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)
	_, err := adaptor.GetRequestURL(info)
	require.Error(t, err)
}

func TestConvertEmbeddingRequest(t *testing.T) {
	t.Parallel()

	dimensions := 0
	outputDimension := 0
	inputType := "search_document"
	tests := []struct {
		name       string
		request    dto.EmbeddingRequest
		wantTexts  []string
		wantError  bool
		errorField string
	}{
		{name: "single string", request: dto.EmbeddingRequest{Input: "first"}, wantTexts: []string{"first"}},
		{name: "string array preserves order", request: dto.EmbeddingRequest{Input: []any{"first", "second"}, EncodingFormat: "float", User: "accepted"}, wantTexts: []string{"first", "second"}},
		{name: "empty string", request: dto.EmbeddingRequest{Input: "  "}, wantError: true, errorField: "input"},
		{name: "empty array", request: dto.EmbeddingRequest{Input: []any{}}, wantError: true, errorField: "input"},
		{name: "mixed array", request: dto.EmbeddingRequest{Input: []any{"first", 2.0}}, wantError: true, errorField: "input[1]"},
		{name: "token array", request: dto.EmbeddingRequest{Input: []any{1.0, 2.0}}, wantError: true, errorField: "input[0]"},
		{name: "base64", request: dto.EmbeddingRequest{Input: "first", EncodingFormat: "base64"}, wantError: true, errorField: "encoding_format"},
		{name: "dimensions explicit zero", request: dto.EmbeddingRequest{Input: "first", Dimensions: &dimensions}, wantError: true, errorField: "dimensions"},
		{name: "output dimension explicit zero", request: dto.EmbeddingRequest{Input: "first", OutputDimension: &outputDimension}, wantError: true, errorField: "output_dimension"},
		{name: "input type", request: dto.EmbeddingRequest{Input: "first", InputType: &inputType}, wantError: true, errorField: "input_type"},
		{name: "provider", request: dto.EmbeddingRequest{Input: "first", Provider: []byte(`{"cloudflare":{"pooling":"mean"}}`)}, wantError: true, errorField: "provider"},
		{name: "extra body", request: dto.EmbeddingRequest{Input: "first", ExtraBody: []byte(`{"cloudflare":{"pooling":"mean"}}`)}, wantError: true, errorField: "extra_body"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			adaptor := &Adaptor{}
			converted, err := adaptor.ConvertEmbeddingRequest(nil, &relaycommon.RelayInfo{}, test.request)
			if test.wantError {
				require.Error(t, err)
				var apiErr *types.NewAPIError
				require.ErrorAs(t, err, &apiErr)
				require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
				require.True(t, types.IsSkipRetryError(apiErr))
				require.Contains(t, err.Error(), test.errorField)
				return
			}
			require.NoError(t, err)
			cfRequest, ok := converted.(*CfEmbeddingRequest)
			require.True(t, ok)
			require.Equal(t, test.wantTexts, cfRequest.Text)
		})
	}
}

func TestConvertEmbeddingRequestRejectsUnknownCloudflareField(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"@cf/baai/bge-m3","input":"text","pooling":"mean"}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	adaptor := &Adaptor{}
	_, err := adaptor.ConvertEmbeddingRequest(ctx, &relaycommon.RelayInfo{}, dto.EmbeddingRequest{Input: "text"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "pooling")

	storage, storageErr := common.GetBodyStorage(ctx)
	require.NoError(t, storageErr)
	require.NoError(t, storage.Close())
}

func TestConvertEmbeddingRequestRejectsExplicitEmptyEncodingFormat(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"@cf/baai/bge-m3","input":"text","encoding_format":""}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	adaptor := &Adaptor{}
	_, err := adaptor.ConvertEmbeddingRequest(ctx, &relaycommon.RelayInfo{}, dto.EmbeddingRequest{Input: "text"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "encoding_format")

	storage, storageErr := common.GetBodyStorage(ctx)
	require.NoError(t, storageErr)
	require.NoError(t, storage.Close())
}
