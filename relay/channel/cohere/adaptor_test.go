package cohere

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCohereAdaptorGetRequestURL(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	testCases := []struct {
		name      string
		relayMode int
		expected  string
		wantErr   bool
	}{
		{
			name:      "embedding uses v2 embed",
			relayMode: relayconstant.RelayModeEmbeddings,
			expected:  "https://api.cohere.ai/v2/embed",
		},
		{
			name:      "rerank uses v2 rerank",
			relayMode: relayconstant.RelayModeRerank,
			expected:  "https://api.cohere.ai/v2/rerank",
		},
		{
			name:      "chat unsupported",
			relayMode: relayconstant.RelayModeChatCompletions,
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
					ChannelBaseUrl: "https://api.cohere.ai",
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

func TestCohereAdaptorRejectsChatConversion(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	out, err := adaptor.ConvertOpenAIRequest(nil, nil, &dto.GeneralOpenAIRequest{Model: "command-r"})
	require.Nil(t, out)
	require.ErrorIs(t, err, unsupportedCohereInterfaceError)
}

func TestCohereConvertEmbeddingRequestToV2(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	inputType := "search_document"
	truncate := "END"
	priority := 3
	maxTokens := 128
	req := dto.EmbeddingRequest{
		Model:           "embed-v4.0",
		Input:           []any{"doc1", "doc2"},
		InputType:       &inputType,
		Dimensions:      common.GetPointer(768),
		OutputDimension: common.GetPointer(1024),
		MaxTokens:       &maxTokens,
		EmbeddingTypes:  []string{"float"},
		Provider:        []byte(`{"cohere":{"priority":3,"truncate":"END"}}`),
	}

	out, err := adaptor.ConvertEmbeddingRequest(nil, &relaycommon.RelayInfo{}, req)
	require.NoError(t, err)
	converted, ok := out.(*dto.CohereV2EmbedRequest)
	require.True(t, ok)
	require.Equal(t, "embed-v4.0", converted.Model)
	require.Equal(t, "search_document", converted.InputType)
	require.Equal(t, []string{"doc1", "doc2"}, converted.Texts)
	require.Equal(t, 1024, *converted.OutputDimension)
	require.Equal(t, maxTokens, *converted.MaxTokens)
	require.Equal(t, truncate, *converted.Truncate)
	require.Equal(t, priority, *converted.Priority)
	require.Equal(t, []string{"float"}, converted.EmbeddingTypes)
}

func TestCohereConvertEmbeddingRejectsMissingInputType(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	out, err := adaptor.ConvertEmbeddingRequest(nil, &relaycommon.RelayInfo{}, dto.EmbeddingRequest{
		Model: "embed-v4.0",
		Input: "doc",
	})
	require.Nil(t, out)
	require.EqualError(t, err, invalidDataFormatMessage)
}

func TestCohereConvertRerankRequestToV2(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	topN := 2
	maxTokens := 512
	priority := 4
	out, err := adaptor.ConvertRerankRequest(nil, relayconstant.RelayModeRerank, dto.RerankRequest{
		Model:           "rerank-v4.0",
		Query:           "q",
		Documents:       []any{"doc1", "doc2"},
		TopN:            &topN,
		MaxTokensPerDoc: &maxTokens,
		Priority:        &priority,
	})
	require.NoError(t, err)
	converted, ok := out.(*dto.CohereV2RerankRequest)
	require.True(t, ok)
	require.Equal(t, "rerank-v4.0", converted.Model)
	require.Equal(t, "q", converted.Query)
	require.Equal(t, []string{"doc1", "doc2"}, converted.Documents)
	require.Equal(t, topN, *converted.TopN)
	require.Equal(t, maxTokens, *converted.MaxTokensPerDoc)
	require.Equal(t, priority, *converted.Priority)
}

func TestCohereConvertRerankRejectsV1OnlyFields(t *testing.T) {
	t.Parallel()

	adaptor := &Adaptor{}
	returnDocuments := true
	out, err := adaptor.ConvertRerankRequest(nil, relayconstant.RelayModeRerank, dto.RerankRequest{
		Model:           "rerank-v4.0",
		Query:           "q",
		Documents:       []any{"doc1"},
		ReturnDocuments: &returnDocuments,
	})
	require.Nil(t, out)
	require.EqualError(t, err, invalidDataFormatMessage)
}

func TestCoherePassthroughHandlerPreservesNativeResponse(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v2/rerank", nil)

	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatCohereRerank,
		RelayMode:   relayconstant.RelayModeRerank,
	}
	info.SetEstimatePromptTokens(9)

	body := []byte(`{"id":"abc","results":[{"index":0,"relevance_score":0.9}],"meta":{"billed_units":{"search_units":1}}}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}

	usage, apiErr := coherePassthroughHandler(c, resp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	require.Equal(t, 9, usage.PromptTokens)
	require.Equal(t, 1, usage.SearchUnits)
	require.JSONEq(t, string(body), w.Body.String())
}

func TestCohereRerankHandlerConvertsV2Response(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank", nil)

	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeRerank,
		OriginModelName: "rerank-v4.0",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "rerank-v4.0",
		},
		RerankerInfo: &relaycommon.RerankerInfo{
			Documents: []any{"doc0", "doc1"},
		},
	}
	body := []byte(`{"results":[{"index":1,"relevance_score":0.7}],"meta":{"billed_units":{"input_tokens":5,"output_tokens":2}}}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}

	usage, apiErr := cohereRerankHandler(c, resp, info)
	require.Nil(t, apiErr)
	require.Equal(t, 5, usage.PromptTokens)
	require.Equal(t, 2, usage.CompletionTokens)
	require.Equal(t, 7, usage.TotalTokens)

	var rerankResp dto.RerankResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &rerankResp))
	require.Len(t, rerankResp.Results, 1)
	require.Equal(t, "doc1", rerankResp.Results[0].Document)
	require.Equal(t, 5, rerankResp.Usage.PromptTokens)
	require.Equal(t, 7, rerankResp.Usage.TotalTokens)
}

func TestCohereEmbeddingHandlerConvertsFloatEmbeddings(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)

	info := &relaycommon.RelayInfo{
		RelayMode:       relayconstant.RelayModeEmbeddings,
		OriginModelName: "embed-v4.0",
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "embed-v4.0",
		},
	}
	body := []byte(`{"embeddings":{"float":[[0.1,0.2],[0.3,0.4]]},"meta":{"billed_units":{"input_tokens":6}}}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}

	usage, apiErr := cohereEmbeddingHandler(c, resp, info)
	require.Nil(t, apiErr)
	require.Equal(t, 6, usage.PromptTokens)

	var embeddingResp dto.OpenAIEmbeddingResponse
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &embeddingResp))
	require.Equal(t, "embed-v4.0", embeddingResp.Model)
	require.Len(t, embeddingResp.Data, 2)
	require.Equal(t, []float64{0.1, 0.2}, embeddingResp.Data[0].Embedding)
	require.Equal(t, 6, embeddingResp.Usage.PromptTokens)
}

func TestCohereEmbeddingHandlerRejectsNonFloatEmbeddings(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", nil)

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeEmbeddings,
	}
	body := []byte(`{"embeddings":{"int8":[[1,2]]},"meta":{"billed_units":{"input_tokens":6}}}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}

	usage, apiErr := cohereEmbeddingHandler(c, resp, info)
	require.Nil(t, usage)
	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeBadResponseBody, apiErr.GetErrorCode())
	require.Empty(t, w.Body.String())
}
