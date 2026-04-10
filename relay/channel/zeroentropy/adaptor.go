package zeroentropy

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseURL := strings.TrimSpace(info.ChannelBaseUrl)
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		return "", errors.New("base url is empty")
	}

	switch info.RelayMode {
	case relayconstant.RelayModeEmbeddings:
		return fmt.Sprintf("%s/v1/models/embed", baseURL), nil
	case relayconstant.RelayModeRerank:
		return fmt.Sprintf("%s/v1/models/rerank", baseURL), nil
	default:
		return "", fmt.Errorf("unsupported relay mode: %d", info.RelayMode)
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)

	if strings.TrimSpace(req.Get("Accept")) == "" {
		req.Set("Accept", "application/json")
	}

	apiKey := strings.TrimSpace(info.ApiKey)
	if apiKey == "" {
		return errors.New("api key is empty")
	}

	req.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("zeroentropy only supports embeddings and rerank")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(*gin.Context, *relaycommon.RelayInfo, dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("zeroentropy only supports embeddings and rerank")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("zeroentropy only supports embeddings and rerank")
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("zeroentropy only supports embeddings and rerank")
}

func (a *Adaptor) ConvertAudioRequest(*gin.Context, *relaycommon.RelayInfo, dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("zeroentropy only supports embeddings and rerank")
}

func (a *Adaptor) ConvertImageRequest(*gin.Context, *relaycommon.RelayInfo, dto.ImageRequest) (any, error) {
	return nil, errors.New("zeroentropy only supports embeddings and rerank")
}

type zeroEntropyEmbeddingRequest struct {
	Input          any    `json:"input"`
	InputType      string `json:"input_type"`
	Model          string `json:"model"`
	Dimensions     *int   `json:"dimensions,omitempty"`
	EncodingFormat string `json:"encoding_format,omitempty"`
	Latency        string `json:"latency,omitempty"`
}

func coerceZeroEntropyEmbeddingInput(input any) (any, string, error) {
	switch v := input.(type) {
	case string:
		return v, "query", nil
	case []any:
		parts := make([]string, 0, len(v))
		for i := range v {
			s, ok := v[i].(string)
			if !ok {
				return nil, "", fmt.Errorf("input[%d] is not a string", i)
			}
			parts = append(parts, s)
		}
		return parts, "document", nil
	case []string:
		return v, "document", nil
	default:
		return nil, "", fmt.Errorf("unsupported input type: %T", input)
	}
}

func (a *Adaptor) ConvertEmbeddingRequest(_ *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	input, inputType, err := coerceZeroEntropyEmbeddingInput(request.Input)
	if err != nil {
		return nil, err
	}

	modelName := strings.TrimSpace(info.UpstreamModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(request.Model)
	}

	return zeroEntropyEmbeddingRequest{
		Input:          input,
		InputType:      inputType,
		Model:          modelName,
		Dimensions:     request.Dimensions,
		EncodingFormat: strings.TrimSpace(request.EncodingFormat),
		Latency:        "fast",
	}, nil
}

type zeroEntropyRerankRequest struct {
	Documents []string `json:"documents"`
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Latency   string   `json:"latency,omitempty"`
	TopN      *int     `json:"top_n,omitempty"`
}

func coerceZeroEntropyDocuments(documents []any) ([]string, error) {
	out := make([]string, 0, len(documents))
	for _, document := range documents {
		switch v := document.(type) {
		case string:
			out = append(out, v)
		case map[string]any:
			if text, ok := v["text"]; ok {
				out = append(out, fmt.Sprintf("%v", text))
			} else {
				out = append(out, fmt.Sprintf("%v", v))
			}
		default:
			out = append(out, fmt.Sprintf("%v", v))
		}
	}
	return out, nil
}

func (a *Adaptor) ConvertRerankRequest(_ *gin.Context, _ int, request dto.RerankRequest) (any, error) {
	documents, err := coerceZeroEntropyDocuments(request.Documents)
	if err != nil {
		return nil, err
	}

	modelName := strings.TrimSpace(request.Model)
	if modelName == "" {
		return nil, errors.New("model is empty")
	}

	topN := request.TopN
	if topN != nil && *topN <= 0 {
		fixed := 1
		topN = &fixed
	}

	return zeroEntropyRerankRequest{
		Documents: documents,
		Model:     modelName,
		Query:     request.Query,
		Latency:   "fast",
		TopN:      topN,
	}, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case relayconstant.RelayModeEmbeddings:
		return zeroEntropyEmbeddingHandler(c, resp, info)
	case relayconstant.RelayModeRerank:
		return zeroEntropyRerankHandler(c, resp, info)
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

