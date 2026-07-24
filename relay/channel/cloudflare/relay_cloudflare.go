package cloudflare

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
)

func convertCf2CompletionsRequest(textRequest dto.GeneralOpenAIRequest) *CfRequest {
	p, _ := textRequest.Prompt.(string)
	return &CfRequest{
		Prompt:      p,
		MaxTokens:   textRequest.GetMaxTokens(),
		Stream:      lo.FromPtrOr(textRequest.Stream, false),
		Temperature: textRequest.Temperature,
	}
}

func cfStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*types.NewAPIError, *dto.Usage) {
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	helper.SetEventStreamHeaders(c)
	id := helper.GetResponseID(c)
	var responseText string
	isFirst := true

	for scanner.Scan() {
		data := scanner.Text()
		if len(data) < len("data: ") {
			continue
		}
		data = strings.TrimPrefix(data, "data: ")
		data = strings.TrimSuffix(data, "\r")

		if data == "[DONE]" {
			break
		}

		var response dto.ChatCompletionsStreamResponse
		err := common.Unmarshal([]byte(data), &response)
		if err != nil {
			logger.LogError(c, "error_unmarshalling_stream_response: "+err.Error())
			continue
		}
		for _, choice := range response.Choices {
			choice.Delta.Role = "assistant"
			responseText += choice.Delta.GetContentString()
		}
		response.Id = id
		response.Model = info.UpstreamModelName
		err = helper.ObjectData(c, response)
		if isFirst {
			isFirst = false
			info.FirstResponseTime = time.Now()
		}
		if err != nil {
			logger.LogError(c, "error_rendering_stream_response: "+err.Error())
		}
	}

	if err := scanner.Err(); err != nil {
		logger.LogError(c, "error_scanning_stream_response: "+err.Error())
	}
	usage := service.ResponseText2Usage(c, responseText, info.UpstreamModelName, info.GetEstimatePromptTokens())
	if info.ShouldIncludeUsage {
		response := helper.GenerateFinalUsageResponse(id, info.StartTime.Unix(), info.UpstreamModelName, *usage)
		err := helper.ObjectData(c, response)
		if err != nil {
			logger.LogError(c, "error_rendering_final_usage_response: "+err.Error())
		}
	}
	helper.Done(c)

	service.CloseResponseBodyGracefully(resp)

	return nil, usage
}

func cfHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*types.NewAPIError, *dto.Usage) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}
	service.CloseResponseBodyGracefully(resp)
	var response dto.TextResponse
	err = common.Unmarshal(responseBody, &response)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}
	response.Model = info.UpstreamModelName
	var responseText string
	for _, choice := range response.Choices {
		responseText += choice.Message.StringContent()
	}
	usage := service.ResponseText2Usage(c, responseText, info.UpstreamModelName, info.GetEstimatePromptTokens())
	response.Usage = *usage
	response.Id = helper.GetResponseID(c)
	jsonResponse, err := common.Marshal(response)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)
	return nil, usage
}

func cfSTTHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*types.NewAPIError, *dto.Usage) {
	var cfResp CfAudioResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}
	service.CloseResponseBodyGracefully(resp)
	err = common.Unmarshal(responseBody, &cfResp)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}

	audioResp := &dto.AudioResponse{
		Text: cfResp.Result.Text,
	}

	jsonResponse, err := common.Marshal(audioResp)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)

	usage := service.ResponseText2Usage(c, cfResp.Result.Text, info.UpstreamModelName, info.GetEstimatePromptTokens())
	return nil, usage
}

func cfEmbeddingHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response, expectedCount int) (*dto.Usage, *types.NewAPIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusBadGateway)
	}
	service.CloseResponseBodyGracefully(resp)

	var cfResp CfEmbeddingResponse
	if err = common.Unmarshal(responseBody, &cfResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if !cfResp.Success {
		return nil, types.NewOpenAIError(
			fmt.Errorf("Cloudflare Workers AI request failed%s", formatCfResponseMessages(cfResp.Errors, cfResp.Messages)),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}
	if err = validateCfEmbeddingResult(cfResp.Result, expectedCount); err != nil {
		return nil, types.NewOpenAIError(
			fmt.Errorf("invalid Cloudflare embedding response: %w%s", err, formatCfResponseMessages(cfResp.Errors, cfResp.Messages)),
			types.ErrorCodeBadResponseBody,
			http.StatusBadGateway,
		)
	}

	usage := service.ResponseText2Usage(c, "", info.UpstreamModelName, info.GetEstimatePromptTokens())
	modelName := info.OriginModelName
	if modelName == "" {
		modelName = info.UpstreamModelName
	}
	openAIResponse := dto.OpenAIEmbeddingResponse{
		Object: "list",
		Data:   make([]dto.OpenAIEmbeddingResponseItem, 0, len(cfResp.Result.Data)),
		Model:  modelName,
		Usage:  *usage,
	}
	for index, embedding := range cfResp.Result.Data {
		openAIResponse.Data = append(openAIResponse.Data, dto.OpenAIEmbeddingResponseItem{
			Object:    "embedding",
			Index:     index,
			Embedding: embedding,
		})
	}

	jsonResponse, err := common.Marshal(openAIResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)
	return usage, nil
}

func validateCfEmbeddingResult(result CfEmbeddingResult, expectedCount int) error {
	if len(result.Data) == 0 {
		return errors.New("missing result.data")
	}
	if len(result.Shape) != 2 || result.Shape[0] <= 0 || result.Shape[1] <= 0 {
		return errors.New("result.shape must contain positive row and dimension counts")
	}
	if result.Shape[0] != len(result.Data) {
		return fmt.Errorf("result.shape row count %d does not match %d vectors", result.Shape[0], len(result.Data))
	}
	if expectedCount > 0 && len(result.Data) != expectedCount {
		return fmt.Errorf("received %d vectors for %d inputs", len(result.Data), expectedCount)
	}
	for index, embedding := range result.Data {
		if len(embedding) == 0 {
			return fmt.Errorf("result.data[%d] is empty", index)
		}
		if len(embedding) != result.Shape[1] {
			return fmt.Errorf("result.data[%d] dimension %d does not match shape %d", index, len(embedding), result.Shape[1])
		}
	}
	return nil
}

func formatCfResponseMessages(groups ...[]CfResponseMessage) string {
	details := make([]string, 0)
	for _, messages := range groups {
		for _, message := range messages {
			parts := make([]string, 0, 2)
			if message.Code != nil {
				parts = append(parts, fmt.Sprintf("code=%v", message.Code))
			}
			if strings.TrimSpace(message.Message) != "" {
				parts = append(parts, fmt.Sprintf("message=%s", message.Message))
			}
			if len(parts) > 0 {
				details = append(details, strings.Join(parts, ", "))
			}
		}
	}
	if len(details) == 0 {
		return ""
	}
	return ": " + strings.Join(details, "; ")
}
