package elastic

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func normalizeInferenceID(modelName string) (string, error) {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return "", errors.New("model is empty")
	}
	if strings.HasPrefix(modelName, ".") {
		return modelName, nil
	}
	if isHostedInferenceModelName(modelName) {
		return "." + modelName, nil
	}
	return modelName, nil
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := strings.TrimSpace(info.ChannelBaseUrl)
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return "", errors.New("base url is empty")
	}

	modelName := strings.TrimSpace(info.UpstreamModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(info.OriginModelName)
	}
	inferenceID, err := normalizeInferenceID(modelName)
	if err != nil {
		return "", err
	}
	escapedInferenceID := url.PathEscape(inferenceID)

	switch info.RelayMode {
	case relayconstant.RelayModeEmbeddings:
		return fmt.Sprintf("%s/_inference/text_embedding/%s", baseURL, escapedInferenceID), nil
	case relayconstant.RelayModeRerank:
		return fmt.Sprintf("%s/_inference/rerank/%s", baseURL, escapedInferenceID), nil
	default:
		return "", fmt.Errorf("unsupported relay mode: %d", info.RelayMode)
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	apiKey := strings.TrimSpace(info.ApiKey)
	if apiKey == "" {
		return errors.New("api key is empty")
	}
	if strings.HasPrefix(strings.ToLower(apiKey), "apikey ") {
		req.Set("Authorization", apiKey)
	} else {
		req.Set("Authorization", "ApiKey "+apiKey)
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("elastic inference endpoints only support embeddings and rerank")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(*gin.Context, *relaycommon.RelayInfo, dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("elastic inference endpoints only support embeddings and rerank")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("elastic inference endpoints only support embeddings and rerank")
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("elastic inference endpoints only support embeddings and rerank")
}

func (a *Adaptor) ConvertAudioRequest(*gin.Context, *relaycommon.RelayInfo, dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("elastic inference endpoints only support embeddings and rerank")
}

func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, errors.New("elastic inference endpoints only support embeddings and rerank")
}

type elasticTextEmbeddingRequest struct {
	Input     any    `json:"input"`
	InputType string `json:"input_type"`
}

func parseEmbeddingInput(input any) (any, error) {
	switch v := input.(type) {
	case string:
		return v, nil
	case []any:
		parts := make([]string, 0, len(v))
		for i := range v {
			s, ok := v[i].(string)
			if !ok {
				return nil, fmt.Errorf("input[%d] is not a string", i)
			}
			parts = append(parts, s)
		}
		return parts, nil
	default:
		return nil, fmt.Errorf("unsupported input type: %T", input)
	}
}

func (a *Adaptor) ConvertEmbeddingRequest(_ *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	modelName := strings.TrimSpace(info.UpstreamModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(request.Model)
	}
	// Elastic hosted inference model names are prefixed with "." in the URL; accept both forms here.
	modelName = strings.TrimPrefix(modelName, ".")

	// NOTE: For Elastic Inference Endpoints, jina-clip-v2 appears to only support single input in practice.
	// If clients send OpenAI-style batch inputs, reject early to avoid upstream 404 and provide a clearer error.
	if strings.EqualFold(modelName, "jina-clip-v2") {
		switch v := request.Input.(type) {
		case []any:
			if len(v) > 1 {
				return nil, types.NewOpenAIError(
					fmt.Errorf("jina-clip-v2 does not support batch input on elastic inference endpoints; provide a single string input"),
					types.ErrorCodeInvalidRequest,
					http.StatusBadRequest,
					types.ErrOptionWithSkipRetry(),
				)
			}
			// Compatibility: some clients (and the admin channel test) send input as a single-element array.
			// Unwrap it to a plain string to match upstream expectations.
			if len(v) == 1 {
				if s, ok := v[0].(string); ok {
					request.Input = s
				}
			}
		}
	}

	input, err := parseEmbeddingInput(request.Input)
	if err != nil {
		return nil, err
	}

	inputType := strings.TrimSpace(lo.FromPtrOr(request.InputType, ""))
	if inputType == "" {
		inputType = "ingest"
	}

	return elasticTextEmbeddingRequest{
		Input:     input,
		InputType: inputType,
	}, nil
}

type elasticRerankRequest struct {
	Query           string   `json:"query"`
	TopN            *int     `json:"top_n,omitempty"`
	ReturnDocuments *bool    `json:"return_documents,omitempty"`
	Input           []string `json:"input"`
}

func (a *Adaptor) ConvertRerankRequest(_ *gin.Context, _ int, request dto.RerankRequest) (any, error) {
	input := make([]string, 0, len(request.Documents))
	for _, document := range request.Documents {
		switch v := document.(type) {
		case string:
			input = append(input, v)
		case map[string]any:
			if text, ok := v["text"]; ok {
				input = append(input, fmt.Sprintf("%v", text))
			} else {
				input = append(input, fmt.Sprintf("%v", v))
			}
		default:
			input = append(input, fmt.Sprintf("%v", v))
		}
	}

	topN := request.TopN
	if topN != nil && *topN <= 0 {
		defaultTopN := 1
		topN = &defaultTopN
	}

	return elasticRerankRequest{
		Query:           request.Query,
		TopN:            topN,
		ReturnDocuments: request.ReturnDocuments,
		Input:           input,
	}, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case relayconstant.RelayModeEmbeddings:
		return elasticEmbeddingHandler(c, resp, info)
	case relayconstant.RelayModeRerank:
		return elasticRerankHandler(c, resp, info)
	default:
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("unsupported relay mode: %d", info.RelayMode),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
