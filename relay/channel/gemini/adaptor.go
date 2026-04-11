package gemini

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/reasoning"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	if len(request.Contents) > 0 {
		for i, content := range request.Contents {
			if i == 0 {
				if request.Contents[0].Role == "" {
					request.Contents[0].Role = "user"
				}
			}
			for _, part := range content.Parts {
				if part.FileData != nil {
					if part.FileData.MimeType == "" && strings.Contains(part.FileData.FileUri, "www.youtube.com") {
						part.FileData.MimeType = "video/webm"
					}
				}
			}
		}
	}
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	adaptor := openai.Adaptor{}
	oaiReq, err := adaptor.ConvertClaudeRequest(c, info, req)
	if err != nil {
		return nil, err
	}
	return a.ConvertOpenAIRequest(c, info, oaiReq.(*dto.GeneralOpenAIRequest))
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if !strings.HasPrefix(info.UpstreamModelName, "imagen") {
		return nil, errors.New("not supported model for image generation, only imagen models are supported")
	}

	// convert size to aspect ratio but allow user to specify aspect ratio
	aspectRatio := "1:1" // default aspect ratio
	size := strings.TrimSpace(request.Size)
	if size != "" {
		if strings.Contains(size, ":") {
			aspectRatio = size
		} else {
			switch size {
			case "256x256", "512x512", "1024x1024":
				aspectRatio = "1:1"
			case "1536x1024":
				aspectRatio = "3:2"
			case "1024x1536":
				aspectRatio = "2:3"
			case "1024x1792":
				aspectRatio = "9:16"
			case "1792x1024":
				aspectRatio = "16:9"
			}
		}
	}

	// build gemini imagen request
	geminiRequest := dto.GeminiImageRequest{
		Instances: []dto.GeminiImageInstance{
			{
				Prompt: request.Prompt,
			},
		},
		Parameters: dto.GeminiImageParameters{
			SampleCount:      int(lo.FromPtrOr(request.N, uint(1))),
			AspectRatio:      aspectRatio,
			PersonGeneration: "allow_adult", // default allow adult
		},
	}

	// Set imageSize when quality parameter is specified
	// Map quality parameter to imageSize (only supported by Standard and Ultra models)
	// quality values: auto, high, medium, low (for gpt-image-1), hd, standard (for dall-e-3)
	// imageSize values: 1K (default), 2K
	// https://ai.google.dev/gemini-api/docs/imagen
	// https://platform.openai.com/docs/api-reference/images/create
	if request.Quality != "" {
		imageSize := "1K" // default
		switch request.Quality {
		case "hd", "high":
			imageSize = "2K"
		case "2K":
			imageSize = "2K"
		case "standard", "medium", "low", "auto", "1K":
			imageSize = "1K"
		default:
			// unknown quality value, default to 1K
			imageSize = "1K"
		}
		geminiRequest.Parameters.ImageSize = imageSize
	}

	return geminiRequest, nil
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {

}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {

	if model_setting.GetGeminiSettings().ThinkingAdapterEnabled &&
		!model_setting.ShouldPreserveThinkingSuffix(info.OriginModelName) {
		// 新增逻辑：处理 -thinking-<budget> 格式
		if strings.Contains(info.UpstreamModelName, "-thinking-") {
			parts := strings.Split(info.UpstreamModelName, "-thinking-")
			info.UpstreamModelName = parts[0]
		} else if strings.HasSuffix(info.UpstreamModelName, "-thinking") { // 旧的适配
			info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-thinking")
		} else if strings.HasSuffix(info.UpstreamModelName, "-nothinking") {
			info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-nothinking")
		} else if baseModel, level, ok := reasoning.TrimEffortSuffix(info.UpstreamModelName); ok && level != "" {
			info.UpstreamModelName = baseModel
		}
	}

	version := model_setting.GetGeminiVersionSetting(info.UpstreamModelName)

	if strings.HasPrefix(info.UpstreamModelName, "imagen") {
		return fmt.Sprintf("%s/%s/models/%s:predict", info.ChannelBaseUrl, version, info.UpstreamModelName), nil
	}

	if strings.HasPrefix(info.UpstreamModelName, "text-embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "gemini-embedding") {
		action := "embedContent"
		if info.IsGeminiBatchEmbedding {
			action = "batchEmbedContents"
		}
		return fmt.Sprintf("%s/%s/models/%s:%s", info.ChannelBaseUrl, version, info.UpstreamModelName, action), nil
	}

	action := "generateContent"
	if info.IsStream {
		action = "streamGenerateContent?alt=sse"
		if info.RelayMode == constant.RelayModeGemini {
			info.DisablePing = true
		}
	}
	return fmt.Sprintf("%s/%s/models/%s:%s", info.ChannelBaseUrl, version, info.UpstreamModelName, action), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("x-goog-api-key", info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	geminiRequest, err := CovertOpenAI2Gemini(c, *request, info)
	if err != nil {
		return nil, err
	}

	return geminiRequest, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	// We always build a batch-style payload with `requests`, so ensure we call the
	// batch endpoint upstream to avoid payload/endpoint mismatches.
	info.IsGeminiBatchEmbedding = true

	// Provider-specific extensions (Gemini) for /v1/embeddings.
	//
	// Standard OpenAI embeddings clients send {"input": "text"} or {"input": ["..."]}.
	// To keep that shape compatible while still allowing Gemini-specific multimodal
	// inputs/config, we accept optional `extra_body.google.*`.
	//
	// Supported:
	// - extra_body.google.requests: array of embedContent-like request objects (content + optional embedContentConfig / outputDimensionality / taskType / title)
	// - extra_body.google.content: single Gemini content object (content.parts[])
	// - extra_body.google.contents: array of Gemini content objects
	type googleEmbeddingExtra struct {
		Requests            []*dto.EmbedContentRequest `json:"requests,omitempty"`
		Content             *dto.GeminiChatContent     `json:"content,omitempty"`
		Contents            []dto.GeminiChatContent    `json:"contents,omitempty"`
		EmbedContentConfig  *dto.EmbedContentConfig    `json:"embedContentConfig,omitempty"`
		EmbedContentConfig2 *dto.EmbedContentConfig    `json:"embed_content_config,omitempty"`
		ConfigCompat        *dto.EmbedContentConfig    `json:"config,omitempty"`
	}
	type embeddingExtraBody struct {
		Google *googleEmbeddingExtra `json:"google,omitempty"`
	}

	var extra embeddingExtraBody
	if len(request.ExtraBody) > 0 {
		if err := common.Unmarshal(request.ExtraBody, &extra); err != nil {
			return nil, fmt.Errorf("invalid extra_body: %w", err)
		}
	}

	extraEmbedContentConfig := (*dto.EmbedContentConfig)(nil)
	if extra.Google != nil {
		extraEmbedContentConfig = extra.Google.EmbedContentConfig
		if extraEmbedContentConfig == nil {
			extraEmbedContentConfig = extra.Google.EmbedContentConfig2
		}
		if extraEmbedContentConfig == nil {
			extraEmbedContentConfig = extra.Google.ConfigCompat
		}
	}

	embedReqs := make([]*dto.EmbedContentRequest, 0)
	if extra.Google != nil {
		if len(extra.Google.Requests) > 0 {
			embedReqs = append(embedReqs, extra.Google.Requests...)
		} else if len(extra.Google.Contents) > 0 {
			for i := range extra.Google.Contents {
				content := extra.Google.Contents[i]
				embedReqs = append(embedReqs, &dto.EmbedContentRequest{
					Content:            content,
					EmbedContentConfig: extraEmbedContentConfig,
				})
			}
		} else if extra.Google.Content != nil {
			embedReqs = append(embedReqs, &dto.EmbedContentRequest{
				Content:            *extra.Google.Content,
				EmbedContentConfig: extraEmbedContentConfig,
			})
		}
	}

	geminiBatch := &dto.GeminiBatchEmbeddingRequest{
		Requests: make([]*dto.GeminiEmbeddingRequest, 0),
	}

	// Prefer extra_body.google.* when provided.
	if len(embedReqs) > 0 {
		for i, embedReq := range embedReqs {
			if embedReq == nil {
				return nil, fmt.Errorf("extra_body.google.requests[%d] is null", i)
			}

			taskType := ""
			title := ""
			dimensions := 0

			if embedReq.EmbedContentConfig != nil {
				if embedReq.EmbedContentConfig.TaskType != nil {
					taskType = strings.TrimSpace(*embedReq.EmbedContentConfig.TaskType)
				}
				if embedReq.EmbedContentConfig.Title != nil {
					title = strings.TrimSpace(*embedReq.EmbedContentConfig.Title)
				}
				if embedReq.EmbedContentConfig.OutputDimensionality != nil {
					dimensions = lo.FromPtrOr(embedReq.EmbedContentConfig.OutputDimensionality, 0)
				}
			}
			if taskType == "" && embedReq.TaskType != nil {
				taskType = strings.TrimSpace(*embedReq.TaskType)
			}
			if title == "" && embedReq.Title != nil {
				title = strings.TrimSpace(*embedReq.Title)
			}
			if dimensions <= 0 && embedReq.OutputDimensionality != nil {
				dimensions = lo.FromPtrOr(embedReq.OutputDimensionality, 0)
			}

			// Fall back to the OpenAI-style `dimensions` parameter if the caller didn't
			// specify a per-item outputDimensionality.
			if dimensions <= 0 {
				dimensions = lo.FromPtrOr(request.Dimensions, 0)
			}

			geminiReq := &dto.GeminiEmbeddingRequest{
				Model:   fmt.Sprintf("models/%s", info.UpstreamModelName),
				Content: embedReq.Content,
			}
			if taskType != "" {
				geminiReq.TaskType = taskType
			}
			if title != "" {
				geminiReq.Title = title
			}

			// https://ai.google.dev/api/embeddings#method:-models.embedcontent
			// Only newer models introduced after 2024 support OutputDimensionality.
			switch info.UpstreamModelName {
			case "text-embedding-004", "gemini-embedding-exp-03-07", "gemini-embedding-001", "gemini-embedding-2-preview":
				if dimensions > 0 {
					geminiReq.OutputDimensionality = dimensions
				}
			}

			geminiBatch.Requests = append(geminiBatch.Requests, geminiReq)
		}

		return geminiBatch, nil
	}

	// Default: plain-text OpenAI-style `input`.
	if request.Input == nil {
		return nil, errors.New("input is required")
	}

	inputs := request.ParseInput()
	if len(inputs) == 0 {
		return nil, errors.New("input is empty")
	}

	for _, input := range inputs {
		geminiReq := &dto.GeminiEmbeddingRequest{
			Model: fmt.Sprintf("models/%s", info.UpstreamModelName),
			Content: dto.GeminiChatContent{
				Parts: []dto.GeminiPart{
					{
						Text: input,
					},
				},
			},
		}

		// https://ai.google.dev/api/embeddings#method:-models.embedcontent
		// Only newer models introduced after 2024 support OutputDimensionality.
		switch info.UpstreamModelName {
		case "text-embedding-004", "gemini-embedding-exp-03-07", "gemini-embedding-001", "gemini-embedding-2-preview":
			dimensions := lo.FromPtrOr(request.Dimensions, 0)
			if dimensions > 0 {
				geminiReq.OutputDimensionality = dimensions
			}
		}

		geminiBatch.Requests = append(geminiBatch.Requests, geminiReq)
	}

	return geminiBatch, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == constant.RelayModeGemini {
		if strings.Contains(info.RequestURLPath, ":embedContent") ||
			strings.Contains(info.RequestURLPath, ":batchEmbedContents") {
			return NativeGeminiEmbeddingHandler(c, resp, info)
		}
		if info.IsStream {
			return GeminiTextGenerationStreamHandler(c, info, resp)
		} else {
			return GeminiTextGenerationHandler(c, info, resp)
		}
	}

	if strings.HasPrefix(info.UpstreamModelName, "imagen") {
		return GeminiImageHandler(c, info, resp)
	}

	// check if the model is an embedding model
	if strings.HasPrefix(info.UpstreamModelName, "text-embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "gemini-embedding") {
		return GeminiEmbeddingHandler(c, info, resp)
	}

	if info.IsStream {
		return GeminiChatStreamHandler(c, info, resp)
	} else {
		return GeminiChatHandler(c, info, resp)
	}

}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
