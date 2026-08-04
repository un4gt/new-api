package cloudflare

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	accountID              string
	apiToken               string
	credentialErr          error
	credentialInitialized  bool
	expectedEmbeddingCount int
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	//TODO implement me
	panic("implement me")
	return nil, nil
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.initCredential(info)
	if info != nil {
		info.UpstreamModelName = resolveUpstreamModel(info.UpstreamModelName)
	}
}

func (a *Adaptor) initCredential(info *relaycommon.RelayInfo) {
	a.credentialInitialized = true
	if info == nil {
		a.credentialErr = errors.New("Cloudflare relay info is nil")
		return
	}
	a.accountID, a.apiToken, a.credentialErr = ParseCredential(info.ApiKey)
	if a.credentialErr == nil {
		// Keep the full logical key in Gin context for multi-key status tracking,
		// while generic request processing only sees the API token.
		info.ApiKey = a.apiToken
	}
}

func (a *Adaptor) ensureCredential(info *relaycommon.RelayInfo) error {
	if !a.credentialInitialized {
		a.initCredential(info)
	}
	return a.credentialErr
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if err := a.ensureCredential(info); err != nil {
		return "", err
	}
	baseURL := strings.TrimRight(info.ChannelBaseUrl, "/")
	switch info.RelayMode {
	case constant.RelayModeChatCompletions:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/v1/chat/completions", baseURL, a.accountID), nil
	case constant.RelayModeEmbeddings:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/run/%s", baseURL, a.accountID, info.UpstreamModelName), nil
	case constant.RelayModeResponses:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/v1/responses", baseURL, a.accountID), nil
	default:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/run/%s", baseURL, a.accountID, info.UpstreamModelName), nil
	}
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	if err := a.ensureCredential(info); err != nil {
		return err
	}
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiToken))
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	switch info.RelayMode {
	case constant.RelayModeCompletions:
		return convertCf2CompletionsRequest(*request), nil
	default:
		return request, nil
	}
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	if err := validateEmbeddingFields(c, request); err != nil {
		return nil, err
	}
	texts, err := normalizeEmbeddingInput(request.Input)
	if err != nil {
		return nil, invalidEmbeddingRequest(err)
	}
	a.expectedEmbeddingCount = len(texts)
	return &CfEmbeddingRequest{Text: texts}, nil
}

var cloudflareEmbeddingAllowedFields = map[string]struct{}{
	"model":           {},
	"input":           {},
	"encoding_format": {},
	"user":            {},
}

func validateEmbeddingFields(c *gin.Context, request dto.EmbeddingRequest) error {
	if c != nil && c.Request != nil && c.Request.Body != nil && c.Request.Body != http.NoBody &&
		strings.HasPrefix(c.Request.Header.Get("Content-Type"), "application/json") {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return invalidEmbeddingRequest(fmt.Errorf("failed to read request body: %w", err))
		}
		body, err := storage.Bytes()
		if err != nil {
			return invalidEmbeddingRequest(fmt.Errorf("failed to read request body: %w", err))
		}
		var raw map[string]any
		if err = common.Unmarshal(body, &raw); err != nil {
			return invalidEmbeddingRequest(fmt.Errorf("invalid JSON request body: %w", err))
		}
		unsupported := make([]string, 0)
		for field := range raw {
			if _, ok := cloudflareEmbeddingAllowedFields[field]; !ok {
				unsupported = append(unsupported, field)
			}
		}
		if len(unsupported) > 0 {
			sort.Strings(unsupported)
			return invalidEmbeddingRequest(fmt.Errorf("Cloudflare embeddings does not support field %q", unsupported[0]))
		}
		if value, exists := raw["encoding_format"]; exists {
			encodingFormat, ok := value.(string)
			if !ok || encodingFormat != "float" {
				return invalidEmbeddingRequest(errors.New("Cloudflare embeddings only supports encoding_format \"float\""))
			}
		}
	}

	if request.EncodingFormat != "" && request.EncodingFormat != "float" {
		return invalidEmbeddingRequest(errors.New("Cloudflare embeddings only supports encoding_format \"float\""))
	}
	unsupportedField := ""
	switch {
	case request.InputType != nil:
		unsupportedField = "input_type"
	case request.Dimensions != nil:
		unsupportedField = "dimensions"
	case request.OutputDimension != nil:
		unsupportedField = "output_dimension"
	case request.MaxTokens != nil:
		unsupportedField = "max_tokens"
	case len(request.EmbeddingTypes) > 0:
		unsupportedField = "embedding_types"
	case request.Truncate != nil:
		unsupportedField = "truncate"
	case request.Priority != nil:
		unsupportedField = "priority"
	case request.Seed != nil:
		unsupportedField = "seed"
	case request.Temperature != nil:
		unsupportedField = "temperature"
	case request.TopP != nil:
		unsupportedField = "top_p"
	case request.FrequencyPenalty != nil:
		unsupportedField = "frequency_penalty"
	case request.PresencePenalty != nil:
		unsupportedField = "presence_penalty"
	case len(request.Provider) > 0:
		unsupportedField = "provider"
	case len(request.ExtraBody) > 0:
		unsupportedField = "extra_body"
	}
	if unsupportedField != "" {
		return invalidEmbeddingRequest(fmt.Errorf("Cloudflare embeddings does not support field %q", unsupportedField))
	}
	return nil
}

func normalizeEmbeddingInput(input any) ([]string, error) {
	validateText := func(value string) (string, error) {
		if strings.TrimSpace(value) == "" {
			return "", errors.New("input must contain non-empty strings")
		}
		return value, nil
	}

	switch value := input.(type) {
	case string:
		text, err := validateText(value)
		if err != nil {
			return nil, err
		}
		return []string{text}, nil
	case []string:
		if len(value) == 0 {
			return nil, errors.New("input must not be empty")
		}
		texts := make([]string, len(value))
		for i, item := range value {
			text, err := validateText(item)
			if err != nil {
				return nil, fmt.Errorf("input[%d]: %w", i, err)
			}
			texts[i] = text
		}
		return texts, nil
	case []any:
		if len(value) == 0 {
			return nil, errors.New("input must not be empty")
		}
		texts := make([]string, len(value))
		for i, item := range value {
			stringValue, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("input[%d] must be a string", i)
			}
			text, err := validateText(stringValue)
			if err != nil {
				return nil, fmt.Errorf("input[%d]: %w", i, err)
			}
			texts[i] = text
		}
		return texts, nil
	default:
		return nil, errors.New("input must be a non-empty string or an array of non-empty strings")
	}
}

func invalidEmbeddingRequest(err error) *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		err,
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	// 添加文件字段
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		return nil, errors.New("file is required")
	}
	defer file.Close()
	// 打开临时文件用于保存上传的文件内容
	requestBody := &bytes.Buffer{}

	// 将上传的文件内容复制到临时文件
	if _, err := io.Copy(requestBody, file); err != nil {
		return nil, err
	}
	return requestBody, nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case constant.RelayModeEmbeddings:
		return cfEmbeddingHandler(c, info, resp, a.expectedEmbeddingCount)
	case constant.RelayModeChatCompletions:
		if info.IsStream {
			err, usage = cfStreamHandler(c, info, resp)
		} else {
			err, usage = cfHandler(c, info, resp)
		}
	case constant.RelayModeResponses:
		if info.IsStream {
			usage, err = openai.OaiResponsesStreamHandler(c, info, resp)
		} else {
			usage, err = openai.OaiResponsesHandler(c, info, resp)
		}
	case constant.RelayModeAudioTranslation:
		fallthrough
	case constant.RelayModeAudioTranscription:
		err, usage = cfSTTHandler(c, info, resp)
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
