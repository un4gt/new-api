package voyageai_mongodb

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdaptorGetRequestURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseURL  string
		mode     int
		model    string
		expected string
	}{
		{
			name:     "text embeddings",
			baseURL:  "https://ai.mongodb.com",
			mode:     relayconstant.RelayModeEmbeddings,
			model:    "voyage-4-large",
			expected: "https://ai.mongodb.com/v1/embeddings",
		},
		{
			name:     "multimodal embeddings",
			baseURL:  "https://ai.mongodb.com/",
			mode:     relayconstant.RelayModeEmbeddings,
			model:    ModelVoyageMultimodal35,
			expected: "https://ai.mongodb.com/v1/multimodalembeddings",
		},
		{
			name:     "rerank with v1 base",
			baseURL:  "https://ai.mongodb.com/v1",
			mode:     relayconstant.RelayModeRerank,
			model:    "rerank-2.5",
			expected: "https://ai.mongodb.com/v1/rerank",
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			info := &relaycommon.RelayInfo{
				RelayMode: test.mode,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl:    test.baseURL,
					UpstreamModelName: test.model,
				},
			}
			url, err := (&Adaptor{}).GetRequestURL(info)
			require.NoError(t, err)
			require.Equal(t, test.expected, url)
		})
	}
}

func TestAdaptorInitAndSetupRequestHeader(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "test-key"},
	}
	adaptor := &Adaptor{}
	adaptor.Init(info)
	require.Equal(t, constant.ChannelBaseURLs[constant.ChannelTypeVoyageAIByMongoDB], info.ChannelBaseUrl)

	header := http.Header{}
	require.NoError(t, adaptor.SetupRequestHeader(ctx, &header, info))
	require.Equal(t, "Bearer test-key", header.Get("Authorization"))
	require.Equal(t, "application/json", header.Get("Content-Type"))
	require.Equal(t, "application/json", header.Get("Accept"))
}

func TestConvertTextEmbeddingRequestPreservesExplicitZeroValues(t *testing.T) {
	t.Parallel()

	dimensions := 0
	truncation := false
	outputDType := ""
	inputType := "query"
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "voyage-4-large",
		},
	}
	converted, err := (&Adaptor{}).ConvertEmbeddingRequest(nil, info, dto.EmbeddingRequest{
		Model:           "alias-model",
		Input:           []any{"first", "second"},
		InputType:       &inputType,
		Dimensions:      common.GetPointer(1024),
		OutputDimension: &dimensions,
		OutputDType:     &outputDType,
		Truncation:      &truncation,
		EncodingFormat:  "float",
	})
	require.NoError(t, err)

	request, ok := converted.(textEmbeddingRequest)
	require.True(t, ok)
	require.Equal(t, "voyage-4-large", request.Model)
	require.Equal(t, 0, *request.OutputDimension)
	require.False(t, *request.Truncation)
	require.Equal(t, "", *request.OutputDType)
	require.Nil(t, request.EncodingFormat)

	body, err := common.Marshal(request)
	require.NoError(t, err)
	require.Contains(t, string(body), `"output_dimension":0`)
	require.Contains(t, string(body), `"output_dtype":""`)
	require.Contains(t, string(body), `"truncation":false`)
}

func TestConvertMultimodalEmbeddingRequest(t *testing.T) {
	t.Parallel()

	truncation := false
	outputEncoding := "base64"
	inputs := []any{
		map[string]any{
			"content": []any{
				map[string]any{"type": "text", "text": "a banana"},
				map[string]any{"type": "image_url", "image_url": "https://example.com/banana.jpg"},
			},
		},
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: ModelVoyageMultimodal35,
		},
	}
	converted, err := (&Adaptor{}).ConvertEmbeddingRequest(nil, info, dto.EmbeddingRequest{
		Model:          ModelVoyageMultimodal35,
		Inputs:         inputs,
		Truncation:     &truncation,
		OutputEncoding: &outputEncoding,
	})
	require.NoError(t, err)

	request, ok := converted.(multimodalEmbeddingRequest)
	require.True(t, ok)
	require.Equal(t, inputs, request.Inputs)
	require.False(t, *request.Truncation)
	require.Equal(t, "base64", *request.OutputEncoding)
}

func TestConvertRerankRequestMapsTopNAndPreservesFalse(t *testing.T) {
	t.Parallel()

	topK := 0
	returnDocuments := false
	truncation := false
	converted, err := (&Adaptor{}).ConvertRerankRequest(nil, relayconstant.RelayModeRerank, dto.RerankRequest{
		Model:           "rerank-2.5",
		Query:           "query",
		Documents:       []any{"first", map[string]any{"text": "second"}},
		TopN:            common.GetPointer(2),
		TopK:            &topK,
		ReturnDocuments: &returnDocuments,
		Truncation:      &truncation,
	})
	require.NoError(t, err)

	request, ok := converted.(rerankRequest)
	require.True(t, ok)
	require.Equal(t, []string{"first", "second"}, request.Documents)
	require.Equal(t, 0, *request.TopK)
	require.False(t, *request.ReturnDocuments)
	require.False(t, *request.Truncation)

	body, err := common.Marshal(request)
	require.NoError(t, err)
	require.Contains(t, string(body), `"top_k":0`)
	require.Contains(t, string(body), `"return_documents":false`)
	require.Contains(t, string(body), `"truncation":false`)
}

func TestVoyageRerankHandlerConvertsResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	upstream := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(bytes.NewBufferString(`{
			"object":"list",
			"data":[{"index":1,"relevance_score":0.91}],
			"model":"rerank-2.5",
			"usage":{"total_tokens":17}
		}`)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeRerank,
		RerankerInfo: &relaycommon.RerankerInfo{
			Documents:       []any{"first", "second"},
			ReturnDocuments: true,
		},
	}
	usage, apiErr := voyageRerankHandler(ctx, upstream, info)
	require.Nil(t, apiErr)
	require.Equal(t, 17, usage.PromptTokens)
	require.Equal(t, 17, usage.TotalTokens)

	var response dto.RerankResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Results, 1)
	require.Equal(t, 1, response.Results[0].Index)
	require.Equal(t, 0.91, response.Results[0].RelevanceScore)
	require.Equal(t, "second", response.Results[0].Document)
}

func TestModelList(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{
		"voyage-4-large",
		"voyage-4",
		"voyage-4-lite",
		"voyage-code-3",
		"voyage-4-nano",
		"voyage-multimodal-3.5",
		"rerank-2.5",
		"rerank-2.5-lite",
	}, ModelList)
}
