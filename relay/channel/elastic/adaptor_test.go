package elastic

import (
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

func TestElasticAdaptorConvertEmbeddingRequestRejectsJinaClipV2Batch(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "jina-clip-v2",
		},
	}

	req := dto.EmbeddingRequest{
		Model: "jina-clip-v2",
		Input: []any{"a", "b"},
	}

	_, err := adaptor.ConvertEmbeddingRequest(nil, info, req)
	require.Error(t, err)

	var apiErr *types.NewAPIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	require.Equal(t, types.ErrorCodeInvalidRequest, apiErr.GetErrorCode())
	require.Contains(t, apiErr.Error(), "does not support batch")
}

func TestElasticAdaptorConvertEmbeddingRequestAllowsJinaClipV2Single(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "jina-clip-v2",
		},
	}

	req := dto.EmbeddingRequest{
		Model: "jina-clip-v2",
		Input: "hello",
	}

	out, err := adaptor.ConvertEmbeddingRequest(nil, info, req)
	require.NoError(t, err)

	converted, ok := out.(elasticTextEmbeddingRequest)
	require.True(t, ok)
	require.Equal(t, "hello", converted.Input)
	require.Equal(t, "ingest", converted.InputType)
}

func TestElasticAdaptorSetupRequestHeaderDefaultsAcceptToJSON(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "test-key",
		},
	}

	headers := http.Header{}
	err := adaptor.SetupRequestHeader(ctx, &headers, info)
	require.NoError(t, err)

	require.Equal(t, "application/json", headers.Get("Accept"))
	require.Equal(t, "application/json", headers.Get("Content-Type"))
	require.Equal(t, "ApiKey test-key", headers.Get("Authorization"))
}

func TestElasticAdaptorSetupRequestHeaderKeepsClientAccept(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Accept", "application/json; charset=utf-8")

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "test-key",
		},
	}

	headers := http.Header{}
	err := adaptor.SetupRequestHeader(ctx, &headers, info)
	require.NoError(t, err)

	require.Equal(t, "application/json; charset=utf-8", headers.Get("Accept"))
	require.Equal(t, "ApiKey test-key", headers.Get("Authorization"))
}

func TestElasticAdaptorConvertEmbeddingRequestUnwrapsJinaClipV2SingleElementArray(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "jina-clip-v2",
		},
	}

	req := dto.EmbeddingRequest{
		Model: "jina-clip-v2",
		Input: []any{"hello"},
	}

	out, err := adaptor.ConvertEmbeddingRequest(nil, info, req)
	require.NoError(t, err)

	converted, ok := out.(elasticTextEmbeddingRequest)
	require.True(t, ok)
	require.Equal(t, "hello", converted.Input)
	require.Equal(t, "ingest", converted.InputType)
}
