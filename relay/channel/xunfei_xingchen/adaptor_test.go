package xunfei_xingchen

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

func TestXunfeiXingchenAdaptorGetRequestURL(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://maas-api.cn-huabei-1.xf-yun.com/v2/",
		},
	}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://maas-api.cn-huabei-1.xf-yun.com/v2/embeddings", url)

	info.RelayMode = relayconstant.RelayModeRerank
	url, err = adaptor.GetRequestURL(info)
	require.NoError(t, err)
	require.Equal(t, "https://maas-api.cn-huabei-1.xf-yun.com/v2/rerank", url)
}

func TestXunfeiXingchenAdaptorSetupRequestHeader(t *testing.T) {
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
	require.Equal(t, "application/json", headers.Get("Content-Type"))
	require.Equal(t, "application/json", headers.Get("Accept"))
	require.Equal(t, "Bearer test-key", headers.Get("Authorization"))
}

func TestXunfeiXingchenAdaptorConvertEmbeddingRequest(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "sde0a5839",
		},
	}
	dimensions := 1024
	req := dto.EmbeddingRequest{
		Model:          "ignored-after-mapping",
		Input:          []any{"a", "b"},
		EncodingFormat: "float",
		Dimensions:     &dimensions,
	}

	out, err := adaptor.ConvertEmbeddingRequest(nil, info, req)
	require.NoError(t, err)

	converted, ok := out.(embeddingRequest)
	require.True(t, ok)
	require.Equal(t, "sde0a5839", converted.Model)
	require.Equal(t, []string{"a", "b"}, converted.Input)
	require.Equal(t, "float", converted.EncodingFormat)
	require.Equal(t, &dimensions, converted.Dimensions)
}

func TestXunfeiXingchenAdaptorConvertRerankRequest(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	req := dto.RerankRequest{
		Model:     "s125c8e0e",
		Query:     "夏天最好吃的水果是？",
		Documents: []any{"西瓜", map[string]any{"text": "草莓"}, 42},
	}

	out, err := adaptor.ConvertRerankRequest(nil, relayconstant.RelayModeRerank, req)
	require.NoError(t, err)

	converted, ok := out.(rerankRequest)
	require.True(t, ok)
	require.Equal(t, "s125c8e0e", converted.Model)
	require.Equal(t, "夏天最好吃的水果是？", converted.Query)
	require.Equal(t, []string{"西瓜", "草莓", "42"}, converted.Documents)
}
