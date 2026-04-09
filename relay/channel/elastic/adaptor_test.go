package elastic

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
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
