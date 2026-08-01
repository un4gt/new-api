package voyageai_mongodb

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

var unsupportedInterfaceError = errors.New("VoyageAIByMongoDB only supports embeddings and rerank")

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	if info != nil && strings.TrimSpace(info.ChannelBaseUrl) == "" {
		info.ChannelBaseUrl = constant.ChannelBaseURLs[constant.ChannelTypeVoyageAIByMongoDB]
	}
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("relay info is nil")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(info.ChannelBaseUrl), "/")
	if baseURL == "" {
		return "", errors.New("base url is empty")
	}
	if strings.HasSuffix(baseURL, "/v1") {
		baseURL = strings.TrimSuffix(baseURL, "/v1")
	}

	switch info.RelayMode {
	case relayconstant.RelayModeEmbeddings:
		if isMultimodalModel(info.UpstreamModelName) {
			return baseURL + "/v1/multimodalembeddings", nil
		}
		return baseURL + "/v1/embeddings", nil
	case relayconstant.RelayModeRerank:
		return baseURL + "/v1/rerank", nil
	default:
		return "", unsupportedInterfaceError
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, header)
	header.Set("Authorization", "Bearer "+info.ApiKey)
	header.Set("Content-Type", "application/json")
	if header.Get("Accept") == "" {
		header.Set("Accept", "application/json")
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeneralOpenAIRequest) (any, error) {
	return nil, unsupportedInterfaceError
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(*gin.Context, *relaycommon.RelayInfo, dto.OpenAIResponsesRequest) (any, error) {
	return nil, unsupportedInterfaceError
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, unsupportedInterfaceError
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, unsupportedInterfaceError
}

func (a *Adaptor) ConvertAudioRequest(*gin.Context, *relaycommon.RelayInfo, dto.AudioRequest) (io.Reader, error) {
	return nil, unsupportedInterfaceError
}

func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, unsupportedInterfaceError
}

type textEmbeddingRequest struct {
	Input           any     `json:"input"`
	Model           string  `json:"model"`
	InputType       *string `json:"input_type,omitempty"`
	Truncation      *bool   `json:"truncation,omitempty"`
	OutputDimension *int    `json:"output_dimension,omitempty"`
	OutputDType     *string `json:"output_dtype,omitempty"`
	EncodingFormat  *string `json:"encoding_format,omitempty"`
}

type multimodalEmbeddingRequest struct {
	Inputs         any     `json:"inputs"`
	Model          string  `json:"model"`
	InputType      *string `json:"input_type,omitempty"`
	Truncation     *bool   `json:"truncation,omitempty"`
	OutputEncoding *string `json:"output_encoding,omitempty"`
}

func (a *Adaptor) ConvertEmbeddingRequest(_ *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	modelName := strings.TrimSpace(request.Model)
	if info != nil && strings.TrimSpace(info.UpstreamModelName) != "" {
		modelName = strings.TrimSpace(info.UpstreamModelName)
	}
	if modelName == "" {
		return nil, errors.New("model is empty")
	}
	if info != nil {
		info.UpstreamModelName = modelName
	}

	if isMultimodalModel(modelName) {
		if request.Dimensions != nil || request.OutputDimension != nil {
			return nil, errors.New("dimensions are not supported by Voyage multimodal embeddings")
		}
		if request.OutputDType != nil {
			return nil, errors.New("output_dtype is not supported by Voyage multimodal embeddings")
		}
		inputs := request.Inputs
		if inputs == nil {
			inputs = request.Input
		}
		normalizedInputs, err := normalizeMultimodalInputs(inputs)
		if err != nil {
			return nil, err
		}
		outputEncoding, err := normalizeEmbeddingEncoding(request.OutputEncoding, request.EncodingFormat)
		if err != nil {
			return nil, fmt.Errorf("output_encoding: %w", err)
		}
		return multimodalEmbeddingRequest{
			Inputs:         normalizedInputs,
			Model:          modelName,
			InputType:      request.InputType,
			Truncation:     request.Truncation,
			OutputEncoding: outputEncoding,
		}, nil
	}

	if request.Inputs != nil {
		return nil, errors.New("inputs is only supported by Voyage multimodal embedding models")
	}
	if err := validateTextEmbeddingInput(request.Input); err != nil {
		return nil, err
	}
	if request.OutputEncoding != nil {
		return nil, errors.New("output_encoding is only supported by Voyage multimodal embedding models")
	}
	encodingFormat, err := normalizeEmbeddingEncoding(nil, request.EncodingFormat)
	if err != nil {
		return nil, fmt.Errorf("encoding_format: %w", err)
	}
	outputDimension := request.OutputDimension
	if outputDimension == nil {
		outputDimension = request.Dimensions
	}
	return textEmbeddingRequest{
		Input:           request.Input,
		Model:           modelName,
		InputType:       request.InputType,
		Truncation:      request.Truncation,
		OutputDimension: outputDimension,
		OutputDType:     request.OutputDType,
		EncodingFormat:  encodingFormat,
	}, nil
}

func isMultimodalModel(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(modelName)), "voyage-multimodal-")
}

func validateTextEmbeddingInput(input any) error {
	switch value := input.(type) {
	case string:
		if value == "" {
			return errors.New("input is empty")
		}
	case []string:
		if len(value) == 0 {
			return errors.New("input is empty")
		}
		for i, item := range value {
			if item == "" {
				return fmt.Errorf("input[%d] is empty", i)
			}
		}
	case []any:
		if len(value) == 0 {
			return errors.New("input is empty")
		}
		for i, item := range value {
			text, ok := item.(string)
			if !ok {
				return fmt.Errorf("input[%d] is not a string", i)
			}
			if text == "" {
				return fmt.Errorf("input[%d] is empty", i)
			}
		}
	default:
		return fmt.Errorf("unsupported input type: %T", input)
	}
	return nil
}

func normalizeMultimodalInputs(inputs any) (any, error) {
	switch value := inputs.(type) {
	case []any:
		if len(value) == 0 {
			return nil, errors.New("inputs is empty")
		}
		return value, nil
	case []map[string]any:
		if len(value) == 0 {
			return nil, errors.New("inputs is empty")
		}
		return value, nil
	case map[string]any:
		return []any{value}, nil
	default:
		return nil, fmt.Errorf("unsupported inputs type: %T", inputs)
	}
}

func normalizeEmbeddingEncoding(providerValue *string, openAIValue string) (*string, error) {
	if providerValue != nil {
		return providerValue, nil
	}
	value := strings.TrimSpace(openAIValue)
	switch value {
	case "", "float":
		return nil, nil
	case "base64":
		return &value, nil
	default:
		return nil, fmt.Errorf("unsupported value %q", value)
	}
}

type rerankRequest struct {
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	Model           string   `json:"model"`
	TopK            *int     `json:"top_k,omitempty"`
	ReturnDocuments *bool    `json:"return_documents,omitempty"`
	Truncation      *bool    `json:"truncation,omitempty"`
}

func (a *Adaptor) ConvertRerankRequest(_ *gin.Context, _ int, request dto.RerankRequest) (any, error) {
	documents := make([]string, 0, len(request.Documents))
	for i, document := range request.Documents {
		switch value := document.(type) {
		case string:
			if value == "" {
				return nil, fmt.Errorf("documents[%d] is empty", i)
			}
			documents = append(documents, value)
		case map[string]any:
			text, ok := value["text"].(string)
			if !ok || text == "" {
				return nil, fmt.Errorf("documents[%d].text must be a non-empty string", i)
			}
			documents = append(documents, text)
		default:
			return nil, fmt.Errorf("documents[%d] is not a string", i)
		}
	}
	if len(documents) == 0 {
		return nil, errors.New("documents is empty")
	}
	if request.Query == "" {
		return nil, errors.New("query is empty")
	}
	if strings.TrimSpace(request.Model) == "" {
		return nil, errors.New("model is empty")
	}
	topK := request.TopK
	if topK == nil {
		topK = request.TopN
	}
	return rerankRequest{
		Query:           request.Query,
		Documents:       documents,
		Model:           request.Model,
		TopK:            topK,
		ReturnDocuments: request.ReturnDocuments,
		Truncation:      request.Truncation,
	}, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case relayconstant.RelayModeEmbeddings:
		return openai.OpenaiEmbeddingHandler(c, info, resp)
	case relayconstant.RelayModeRerank:
		return voyageRerankHandler(c, resp, info)
	default:
		return nil, types.NewOpenAIError(unsupportedInterfaceError, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
