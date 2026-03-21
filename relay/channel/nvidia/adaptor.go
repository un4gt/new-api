package nvidia

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

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	if info.ChannelBaseUrl == "" {
		info.ChannelBaseUrl = constant.ChannelBaseURLs[constant.ChannelTypeNvidia]
	}
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", errors.New("relay info is nil")
	}
	if info.RelayMode != relayconstant.RelayModeEmbeddings {
		return "", errors.New("invalid relay mode")
	}
	if IsNVDinoV2Model(info.UpstreamModelName) {
		return nvDinoV2InferURL, nil
	}
	baseURL := strings.TrimSpace(info.ChannelBaseUrl)
	baseURL = strings.TrimSuffix(baseURL, "/")
	if strings.HasSuffix(baseURL, "/v1") {
		return fmt.Sprintf("%s/embeddings", baseURL), nil
	}
	return fmt.Sprintf("%s/v1/embeddings", baseURL), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", fmt.Sprintf("Bearer %s", info.ApiKey))
	if req.Get("Accept") == "" {
		req.Set("Accept", "application/json")
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	modelName := strings.TrimSpace(info.UpstreamModelName)
	if modelName == "" {
		modelName = strings.TrimSpace(request.Model)
	}
	canonicalModelName, ok := NormalizeModel(modelName)
	if !ok {
		return nil, fmt.Errorf("unsupported Nvidia embedding model: %s", modelName)
	}
	info.UpstreamModelName = canonicalModelName
	request.Model = canonicalModelName

	if !IsNVDinoV2Model(canonicalModelName) {
		return request, nil
	}

	nvReq, runtimeHeaders, err := buildNVDinoV2RequestAndRuntimeHeaders(c, info, request.Input)
	if err != nil {
		return nil, err
	}
	if len(runtimeHeaders) > 0 {
		baseHeaders := relaycommon.GetEffectiveHeaderOverride(info)
		merged := make(map[string]any, len(baseHeaders)+len(runtimeHeaders))
		for key, value := range baseHeaders {
			merged[strings.ToLower(strings.TrimSpace(key))] = value
		}
		for key, value := range runtimeHeaders {
			merged[strings.ToLower(strings.TrimSpace(key))] = value
		}
		info.RuntimeHeadersOverride = merged
		info.UseRuntimeHeadersOverride = true
	}
	return nvReq, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info == nil {
		return nil, types.NewError(errors.New("relay info is nil"), types.ErrorCodeInvalidRequest)
	}
	if info.RelayMode != relayconstant.RelayModeEmbeddings {
		return nil, types.NewError(errors.New("invalid relay mode"), types.ErrorCodeInvalidRequest)
	}

	if IsNVDinoV2Model(info.UpstreamModelName) {
		return nvDinoV2EmbeddingHandler(c, resp, info)
	}

	return openai.OpenaiHandler(c, info, resp)
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
