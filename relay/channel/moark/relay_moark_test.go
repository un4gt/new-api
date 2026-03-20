package moark

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type errReadCloser struct{}

func (e errReadCloser) Read(_ []byte) (int, error) {
	return 0, errors.New("read failed")
}

func (e errReadCloser) Close() error {
	return nil
}

func TestSentenceSimilarityHandler_Success(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/sentence-similarity", nil)

	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(9)

	respBody := []byte(`[0.11,0.22,0.33]`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}

	usage, newAPIError := SentenceSimilarityHandler(ctx, resp, info)
	require.NotNil(t, usage)
	require.Nil(t, newAPIError)
	require.Equal(t, 9, usage.PromptTokens)
	require.Equal(t, 9, usage.TotalTokens)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	require.JSONEq(t, string(respBody), recorder.Body.String())
}

func TestSentenceSimilarityHandler_BadResponseBody(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/sentence-similarity", nil)

	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(9)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"score":0.9}`))),
	}

	usage, newAPIError := SentenceSimilarityHandler(ctx, resp, info)
	require.Nil(t, usage)
	require.NotNil(t, newAPIError)
}

func TestRerankMultimodalHandler_Success(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank/multimodal", nil)

	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(9)

	respBody := []byte(`[{"index":0,"score":0.88,"document":{"text":"abc"}}]`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader(respBody)),
	}

	usage, newAPIError := RerankMultimodalHandler(ctx, resp, info)
	require.NotNil(t, usage)
	require.Nil(t, newAPIError)
	require.Equal(t, 9, usage.PromptTokens)
	require.Equal(t, 9, usage.TotalTokens)
	require.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	require.JSONEq(t, string(respBody), recorder.Body.String())
}

func TestRerankMultimodalHandler_BadResponseBody(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank/multimodal", nil)

	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(9)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(`{"index":1,"score":0.8}`))),
	}

	usage, newAPIError := RerankMultimodalHandler(ctx, resp, info)
	require.Nil(t, usage)
	require.NotNil(t, newAPIError)
}

func TestSentenceSimilarityHandler_ReadBodyError(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/sentence-similarity", nil)

	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(9)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       errReadCloser{},
	}

	usage, newAPIError := SentenceSimilarityHandler(ctx, resp, info)
	require.Nil(t, usage)
	require.NotNil(t, newAPIError)
}

func TestRerankMultimodalHandler_ReadBodyError(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/rerank/multimodal", nil)

	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(9)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       errReadCloser{},
	}

	usage, newAPIError := RerankMultimodalHandler(ctx, resp, info)
	require.Nil(t, usage)
	require.NotNil(t, newAPIError)
}
