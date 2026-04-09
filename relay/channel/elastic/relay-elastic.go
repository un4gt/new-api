package elastic

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type elasticTextEmbeddingResponse struct {
	TextEmbedding []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"text_embedding"`

	TextEmbeddingBytes []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"text_embedding_bytes"`

	TextEmbeddingBits []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"text_embedding_bits"`
}

func selectElasticEmbeddings(resp *elasticTextEmbeddingResponse) [][]float64 {
	if resp == nil {
		return nil
	}
	if len(resp.TextEmbedding) > 0 {
		out := make([][]float64, 0, len(resp.TextEmbedding))
		for _, item := range resp.TextEmbedding {
			out = append(out, item.Embedding)
		}
		return out
	}
	if len(resp.TextEmbeddingBytes) > 0 {
		out := make([][]float64, 0, len(resp.TextEmbeddingBytes))
		for _, item := range resp.TextEmbeddingBytes {
			out = append(out, item.Embedding)
		}
		return out
	}
	if len(resp.TextEmbeddingBits) > 0 {
		out := make([][]float64, 0, len(resp.TextEmbeddingBits))
		for _, item := range resp.TextEmbeddingBits {
			out = append(out, item.Embedding)
		}
		return out
	}
	return nil
}

func elasticEmbeddingHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var elasticResp elasticTextEmbeddingResponse
	if err := common.Unmarshal(responseBody, &elasticResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	embeddings := selectElasticEmbeddings(&elasticResp)
	if len(embeddings) == 0 {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid elastic embedding response: missing embeddings"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	promptTokens := info.GetEstimatePromptTokens()
	if promptTokens <= 0 {
		promptTokens = 1
	}
	usage := &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: 0,
		TotalTokens:      promptTokens,
	}

	openAIResponse := dto.OpenAIEmbeddingResponse{
		Object: "list",
		Data:   make([]dto.OpenAIEmbeddingResponseItem, 0, len(embeddings)),
		Model:  info.UpstreamModelName,
		Usage:  *usage,
	}
	for i, emb := range embeddings {
		openAIResponse.Data = append(openAIResponse.Data, dto.OpenAIEmbeddingResponseItem{
			Object:    "embedding",
			Index:     i,
			Embedding: emb,
		})
	}

	out, err := common.Marshal(openAIResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, out)
	return usage, nil
}

type elasticRerankResponse struct {
	Rerank []elasticRerankItem `json:"rerank"`
}

type elasticRerankItem struct {
	Index          any     `json:"index"`
	RelevanceScore any     `json:"relevance_score"`
	Text           *string `json:"text,omitempty"`
}

func parseElasticRerankIndex(value any) (int, error) {
	switch v := value.(type) {
	case float64:
		return int(v), nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, err
		}
		return int(parsed), nil
	default:
		return 0, fmt.Errorf("unsupported index type: %T", value)
	}
}

func parseElasticRerankScore(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported relevance_score type: %T", value)
	}
}

func elasticRerankHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	var elasticResp elasticRerankResponse
	if err := common.Unmarshal(responseBody, &elasticResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if len(elasticResp.Rerank) == 0 {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid elastic rerank response: missing rerank results"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	results := make([]dto.RerankResponseResult, 0, len(elasticResp.Rerank))
	for _, item := range elasticResp.Rerank {
		index, err := parseElasticRerankIndex(item.Index)
		if err != nil {
			return nil, types.NewOpenAIError(fmt.Errorf("invalid elastic rerank result index: %w", err), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		score, err := parseElasticRerankScore(item.RelevanceScore)
		if err != nil {
			return nil, types.NewOpenAIError(fmt.Errorf("invalid elastic rerank result relevance_score: %w", err), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}

		result := dto.RerankResponseResult{
			Index:          index,
			RelevanceScore: score,
		}
		if info.ReturnDocuments {
			if index >= 0 && index < len(info.Documents) {
				result.Document = info.Documents[index]
			} else if item.Text != nil {
				result.Document = *item.Text
			}
		}
		results = append(results, result)
	}

	promptTokens := info.GetEstimatePromptTokens()
	if promptTokens <= 0 {
		promptTokens = 1
	}
	usage := &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: 0,
		TotalTokens:      promptTokens,
	}

	response := dto.RerankResponse{
		Results: results,
		Usage:   *usage,
	}
	out, err := common.Marshal(response)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, out)
	return usage, nil
}
