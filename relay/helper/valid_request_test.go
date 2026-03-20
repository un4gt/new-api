package helper

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newJSONContext(t *testing.T, body any) *gin.Context {
	t.Helper()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	bodyBytes, err := common.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/v1/test", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	ctx.Request = req

	return ctx
}

func TestGetAndValidateSentenceSimilarityRequest_Success(t *testing.T) {
	t.Parallel()

	ctx := newJSONContext(t, map[string]any{
		"model": "bge-m3",
		"inputs": map[string]any{
			"source_sentence": "query",
			"sentences":       []string{"doc1", "doc2"},
		},
		"normalize": false,
	})

	req, err := GetAndValidateSentenceSimilarityRequest(ctx)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, "bge-m3", req.Model)
	require.Equal(t, "query", req.Inputs.SourceSentence)
	require.Equal(t, []string{"doc1", "doc2"}, req.Inputs.Sentences)
	require.NotNil(t, req.Normalize)
	require.False(t, *req.Normalize)
}

func TestGetAndValidateSentenceSimilarityRequest_MissingModel(t *testing.T) {
	t.Parallel()

	ctx := newJSONContext(t, map[string]any{
		"inputs": map[string]any{
			"source_sentence": "query",
			"sentences":       []string{"doc1"},
		},
	})

	req, err := GetAndValidateSentenceSimilarityRequest(ctx)
	require.Nil(t, req)
	require.Error(t, err)
	require.ErrorContains(t, err, "model is empty")
}

func TestGetAndValidateSentenceSimilarityRequest_MissingInputs(t *testing.T) {
	t.Parallel()

	ctx := newJSONContext(t, map[string]any{
		"model": "bge-m3",
		"inputs": map[string]any{
			"source_sentence": "",
			"sentences":       []string{},
		},
	})

	req, err := GetAndValidateSentenceSimilarityRequest(ctx)
	require.Nil(t, req)
	require.Error(t, err)
	require.ErrorContains(t, err, "inputs.source_sentence is empty")
}

func TestGetAndValidateRerankMultimodalRequest_Success(t *testing.T) {
	t.Parallel()

	ctx := newJSONContext(t, map[string]any{
		"model": "bge-reranker-v2-m3",
		"query": map[string]any{
			"text": "what is in this image?",
		},
		"documents": []map[string]any{
			{"text": "a cat on the sofa"},
			{"image": "https://example.com/cat.jpg"},
		},
		"return_documents": false,
	})

	req, err := GetAndValidateRerankMultimodalRequest(ctx)
	require.NoError(t, err)
	require.NotNil(t, req)
	require.Equal(t, "bge-reranker-v2-m3", req.Model)
	require.Len(t, req.Documents, 2)
	require.NotNil(t, req.ReturnDocuments)
	require.False(t, *req.ReturnDocuments)
}

func TestGetAndValidateRerankMultimodalRequest_QueryHasBothTextAndImage(t *testing.T) {
	t.Parallel()

	ctx := newJSONContext(t, map[string]any{
		"model": "bge-reranker-v2-m3",
		"query": map[string]any{
			"text":  "query",
			"image": "https://example.com/query.jpg",
		},
		"documents": []map[string]any{
			{"text": "doc"},
		},
	})

	req, err := GetAndValidateRerankMultimodalRequest(ctx)
	require.Nil(t, req)
	require.Error(t, err)
	require.ErrorContains(t, err, "query must contain exactly one of text or image")
}

func TestGetAndValidateRerankMultimodalRequest_DocumentsExceedLimit(t *testing.T) {
	t.Parallel()

	documents := make([]map[string]any, 26)
	for i := range documents {
		documents[i] = map[string]any{"text": "doc"}
	}

	ctx := newJSONContext(t, map[string]any{
		"model":     "bge-reranker-v2-m3",
		"query":     map[string]any{"text": "query"},
		"documents": documents,
	})

	req, err := GetAndValidateRerankMultimodalRequest(ctx)
	require.Nil(t, req)
	require.Error(t, err)
	require.ErrorContains(t, err, "documents exceeds max items: 25")
}

func TestGetAndValidateRerankMultimodalRequest_DocumentInvalidOneOf(t *testing.T) {
	t.Parallel()

	ctx := newJSONContext(t, map[string]any{
		"model": "bge-reranker-v2-m3",
		"query": map[string]any{
			"text": "query",
		},
		"documents": []map[string]any{
			{},
		},
	})

	req, err := GetAndValidateRerankMultimodalRequest(ctx)
	require.Nil(t, req)
	require.Error(t, err)
	require.ErrorContains(t, err, "documents[0] must contain exactly one of text or image")
}
