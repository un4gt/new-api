package relay

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func EmbeddingHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	var request dto.Request
	switch req := info.Request.(type) {
	case *dto.EmbeddingRequest:
		copied, err := common.DeepCopy(req)
		if err != nil {
			return types.NewError(fmt.Errorf("failed to copy request to EmbeddingRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
		request = copied
	case *dto.CohereV2EmbedRequest:
		copied, err := common.DeepCopy(req)
		if err != nil {
			return types.NewError(fmt.Errorf("failed to copy request to CohereV2EmbedRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
		request = copied
	default:
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected embedding request, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	err := helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}
	if info.RelayFormat == types.RelayFormatCohereEmbed && info.ChannelType != constant.ChannelTypeCohere {
		return types.NewErrorWithStatusCode(fmt.Errorf("cohere v2 embed endpoint requires a Cohere channel"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	// Validate embedding-2 output dimensionality early (Gemini uses outputDimensionality).
	if embeddingRequest, ok := request.(*dto.EmbeddingRequest); ok && isGeminiEmbedding2PreviewModel(info.UpstreamModelName) && embeddingRequest.Dimensions != nil && *embeddingRequest.Dimensions > embedding2MaxOutputDimensionality {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("dimensions must be between 1 and %d", embedding2MaxOutputDimensionality),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	convertedRequest, err := convertEmbeddingRequestByType(c, adaptor, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)

	// For gemini-embedding-2-preview, apply the same multimodal validations/file handling
	// to OpenAI-style /v1/embeddings requests (which we convert to :batchEmbedContents upstream).
	if isGeminiEmbedding2PreviewModel(info.UpstreamModelName) {
		if batchReq, ok := convertedRequest.(*dto.GeminiBatchEmbeddingRequest); ok {
			_, newAPIError := validateAndNormalizeEmbedding2BatchEmbeddingRequest(c, batchReq)
			if newAPIError != nil {
				return newAPIError
			}
		}
	}
	jsonData, err := common.Marshal(convertedRequest)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return newAPIErrorFromParamOverride(err)
		}
	}

	logger.LogDebug(c, fmt.Sprintf("converted embedding request body: %s", string(jsonData)))
	requestBody := bytes.NewBuffer(jsonData)
	statusCodeMappingStr := c.GetString("status_code_mapping")
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}
	postConsumeQuota(c, info, usage.(*dto.Usage))
	return nil
}

func convertEmbeddingRequestByType(c *gin.Context, adaptor channel.Adaptor, info *relaycommon.RelayInfo, request dto.Request) (any, error) {
	switch req := request.(type) {
	case *dto.EmbeddingRequest:
		return adaptor.ConvertEmbeddingRequest(c, info, *req)
	case *dto.CohereV2EmbedRequest:
		return req, nil
	default:
		return nil, fmt.Errorf("invalid embedding request type: %T", request)
	}
}

func isGeminiEmbedding2PreviewModel(modelName string) bool {
	return modelName == "gemini-embedding-2-preview" || modelName == constant.OpenRouterGeminiEmbedding2PreviewModel
}
