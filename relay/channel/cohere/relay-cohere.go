package cohere

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const invalidDataFormatMessage = "无效的数据格式"

var unsupportedCohereInterfaceError = errors.New("cohere only supports embeddings, rerank, and models endpoints")

func requestOpenAIEmbedding2CohereV2(embeddingRequest dto.EmbeddingRequest) (*dto.CohereV2EmbedRequest, error) {
	if err := validateCohereV1CompatibleEmbeddingRequest(embeddingRequest); err != nil {
		return nil, err
	}
	texts := embeddingRequest.ParseInput()
	if len(texts) == 0 {
		return nil, errors.New(invalidDataFormatMessage)
	}
	if embeddingRequest.InputType == nil || strings.TrimSpace(*embeddingRequest.InputType) == "" {
		return nil, errors.New(invalidDataFormatMessage)
	}

	cohereReq := &dto.CohereV2EmbedRequest{
		Model:           embeddingRequest.Model,
		InputType:       strings.TrimSpace(*embeddingRequest.InputType),
		Texts:           texts,
		OutputDimension: cohereOutputDimension(embeddingRequest),
		MaxTokens:       embeddingRequest.MaxTokens,
		Truncate:        embeddingRequest.Truncate,
		EmbeddingTypes:  cohereEmbeddingTypes(embeddingRequest),
		Priority:        embeddingRequest.Priority,
	}
	if len(embeddingRequest.Provider) > 0 {
		if err := applyCohereEmbeddingProviderFields(cohereReq, embeddingRequest.Provider); err != nil {
			return nil, err
		}
	}
	return cohereReq, nil
}

func cohereOutputDimension(embeddingRequest dto.EmbeddingRequest) *int {
	if embeddingRequest.OutputDimension != nil {
		return embeddingRequest.OutputDimension
	}
	return embeddingRequest.Dimensions
}

func cohereEmbeddingTypes(embeddingRequest dto.EmbeddingRequest) []string {
	if len(embeddingRequest.EmbeddingTypes) > 0 {
		return embeddingRequest.EmbeddingTypes
	}
	return []string{"float"}
}

func validateCohereV1CompatibleEmbeddingRequest(embeddingRequest dto.EmbeddingRequest) error {
	if len(embeddingRequest.ExtraBody) > 0 {
		type extraBody struct {
			Cohere *struct {
				Texts any `json:"texts,omitempty"`
			} `json:"cohere,omitempty"`
		}
		var extra extraBody
		if err := common.Unmarshal(embeddingRequest.ExtraBody, &extra); err == nil && extra.Cohere != nil && extra.Cohere.Texts != nil {
			return errors.New(invalidDataFormatMessage)
		}
	}
	if embeddingRequest.Input != nil {
		if inputMap, ok := embeddingRequest.Input.(map[string]any); ok {
			if _, hasTexts := inputMap["texts"]; hasTexts {
				return errors.New(invalidDataFormatMessage)
			}
		}
	}
	return nil
}

func applyCohereEmbeddingProviderFields(cohereReq *dto.CohereV2EmbedRequest, providerRaw []byte) error {
	type cohereProvider struct {
		MaxTokens       *int     `json:"max_tokens,omitempty"`
		OutputDimension *int     `json:"output_dimension,omitempty"`
		EmbeddingTypes  []string `json:"embedding_types,omitempty"`
		Truncate        *string  `json:"truncate,omitempty"`
		Priority        *int     `json:"priority,omitempty"`
		Texts           any      `json:"texts,omitempty"`
	}
	type provider struct {
		Cohere *cohereProvider `json:"cohere,omitempty"`
	}
	var parsed provider
	if err := common.Unmarshal(providerRaw, &parsed); err != nil {
		return err
	}
	if parsed.Cohere == nil {
		return nil
	}
	if parsed.Cohere.Texts != nil {
		return errors.New(invalidDataFormatMessage)
	}
	if parsed.Cohere.MaxTokens != nil {
		cohereReq.MaxTokens = parsed.Cohere.MaxTokens
	}
	if parsed.Cohere.OutputDimension != nil {
		cohereReq.OutputDimension = parsed.Cohere.OutputDimension
	}
	if len(parsed.Cohere.EmbeddingTypes) > 0 {
		cohereReq.EmbeddingTypes = parsed.Cohere.EmbeddingTypes
	}
	if parsed.Cohere.Truncate != nil {
		cohereReq.Truncate = parsed.Cohere.Truncate
	}
	if parsed.Cohere.Priority != nil {
		cohereReq.Priority = parsed.Cohere.Priority
	}
	return nil
}

func requestConvertRerank2CohereV2(rerankRequest dto.RerankRequest) (*dto.CohereV2RerankRequest, error) {
	if err := validateCohereV1CompatibleRerankRequest(rerankRequest); err != nil {
		return nil, err
	}
	documents := make([]string, 0, len(rerankRequest.Documents))
	for _, document := range rerankRequest.Documents {
		doc, ok := document.(string)
		if !ok {
			return nil, errors.New(invalidDataFormatMessage)
		}
		documents = append(documents, doc)
	}
	return &dto.CohereV2RerankRequest{
		Model:           rerankRequest.Model,
		Query:           rerankRequest.Query,
		Documents:       documents,
		TopN:            rerankRequest.TopN,
		MaxTokensPerDoc: rerankRequest.MaxTokensPerDoc,
		Priority:        rerankRequest.Priority,
	}, nil
}

func validateCohereV1CompatibleRerankRequest(rerankRequest dto.RerankRequest) error {
	if rerankRequest.ReturnDocuments != nil ||
		rerankRequest.MaxChunkPerDoc != nil ||
		rerankRequest.OverLapTokens != nil {
		return errors.New(invalidDataFormatMessage)
	}
	return nil
}

func coherePassthroughHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	usage, err := usageFromCohereResponseBody(responseBody, info)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}

func cohereEmbeddingHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	var cohereResp CohereEmbedResponseResult
	if err = common.Unmarshal(responseBody, &cohereResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	embeddings := cohereResp.Embeddings.Float
	if len(embeddings) == 0 {
		embeddings = cohereResp.Embeddings.Float_
	}
	if len(embeddings) == 0 {
		return nil, types.NewOpenAIError(
			fmt.Errorf("invalid cohere embedding response: missing float embeddings"),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}

	usage := usageFromCohereMeta(cohereResp.Meta, info)
	openAIResponse := dto.OpenAIEmbeddingResponse{
		Object: "list",
		Data:   make([]dto.OpenAIEmbeddingResponseItem, 0, len(embeddings)),
		Model:  info.UpstreamModelName,
		Usage:  *usage,
	}
	for i, embedding := range embeddings {
		openAIResponse.Data = append(openAIResponse.Data, dto.OpenAIEmbeddingResponseItem{
			Object:    "embedding",
			Index:     i,
			Embedding: embedding,
		})
	}
	jsonResponse, err := common.Marshal(openAIResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)
	return usage, nil
}

func cohereRerankHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	var cohereResp CohereRerankResponseResult
	if err = common.Unmarshal(responseBody, &cohereResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	usage := usageFromCohereMeta(cohereResp.Meta, info)
	rerankResp := dto.RerankResponse{
		Results: cohereResp.Results,
		Usage:   *usage,
	}
	if info.RerankerInfo != nil && len(info.Documents) > 0 {
		for i := range rerankResp.Results {
			index := rerankResp.Results[i].Index
			if index < 0 || index >= len(info.Documents) {
				continue
			}
			if rerankResp.Results[i].Document == nil {
				rerankResp.Results[i].Document = info.Documents[index]
			}
		}
	}

	jsonResponse, err := common.Marshal(rerankResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)
	return usage, nil
}

func usageFromCohereResponseBody(responseBody []byte, info *relaycommon.RelayInfo) (*dto.Usage, error) {
	var response struct {
		Meta CohereMeta `json:"meta"`
	}
	if err := common.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}
	return usageFromCohereMeta(response.Meta, info), nil
}

func usageFromCohereMeta(meta CohereMeta, info *relaycommon.RelayInfo) *dto.Usage {
	usage := &dto.Usage{
		PromptTokens:     cohereNumberToInt(meta.BilledUnits.InputTokens),
		CompletionTokens: cohereNumberToInt(meta.BilledUnits.OutputTokens),
		SearchUnits:      cohereNumberToInt(meta.BilledUnits.SearchUnits),
	}
	if usage.PromptTokens == 0 {
		usage.PromptTokens = cohereNumberToInt(meta.Tokens.InputTokens)
	}
	if usage.CompletionTokens == 0 {
		usage.CompletionTokens = cohereNumberToInt(meta.Tokens.OutputTokens)
	}
	imageTokens := cohereNumberToInt(meta.BilledUnits.ImageTokens)
	if imageTokens > 0 {
		usage.PromptTokensDetails.ImageTokens = imageTokens
	}
	if usage.PromptTokens == 0 && imageTokens > 0 {
		usage.PromptTokens = imageTokens
	}
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 {
		estimate := info.GetEstimatePromptTokens()
		if estimate <= 0 {
			estimate = 1
		}
		usage.PromptTokens = estimate
	}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return usage
}

func cohereNumberToInt(value *float64) int {
	if value == nil {
		return 0
	}
	if *value <= 0 {
		return 0
	}
	return int(math.Ceil(*value))
}
