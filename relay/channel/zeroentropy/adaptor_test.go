package zeroentropy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestZeroEntropyAdaptorConvertEmbeddingRequestInferInputTypeQuery(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "zembed-1",
		},
	}

	req := dto.EmbeddingRequest{
		Model:          "zembed-1",
		Input:          "hello",
		EncodingFormat: "float",
	}

	out, err := adaptor.ConvertEmbeddingRequest(nil, info, req)
	require.NoError(t, err)

	converted, ok := out.(zeroEntropyEmbeddingRequest)
	require.True(t, ok)
	require.Equal(t, "hello", converted.Input)
	require.Equal(t, "query", converted.InputType)
	require.Equal(t, "zembed-1", converted.Model)
	require.Equal(t, "float", converted.EncodingFormat)
	require.Equal(t, "fast", converted.Latency)
}

func TestZeroEntropyAdaptorConvertEmbeddingRequestInferInputTypeDocument(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "zembed-1",
		},
	}

	req := dto.EmbeddingRequest{
		Model: "zembed-1",
		Input: []any{"a", "b"},
	}

	out, err := adaptor.ConvertEmbeddingRequest(nil, info, req)
	require.NoError(t, err)

	converted, ok := out.(zeroEntropyEmbeddingRequest)
	require.True(t, ok)
	require.Equal(t, "document", converted.InputType)
	require.Equal(t, []string{"a", "b"}, converted.Input)
	require.Equal(t, "fast", converted.Latency)
}

func TestZeroEntropyAdaptorSetupRequestHeader(t *testing.T) {
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
	require.Equal(t, "Bearer test-key", headers.Get("Authorization"))
}

func TestZeroEntropyAdaptorConvertRerankRequest(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	req := dto.RerankRequest{
		Model:     "zerank-2",
		Query:     "q",
		Documents: []any{"d1", "d2"},
	}

	out, err := adaptor.ConvertRerankRequest(nil, relayconstant.RelayModeRerank, req)
	require.NoError(t, err)

	converted, ok := out.(zeroEntropyRerankRequest)
	require.True(t, ok)
	require.Equal(t, []string{"d1", "d2"}, converted.Documents)
	require.Equal(t, "zerank-2", converted.Model)
	require.Equal(t, "q", converted.Query)
	require.Equal(t, "fast", converted.Latency)
	require.Nil(t, converted.TopN)
}

