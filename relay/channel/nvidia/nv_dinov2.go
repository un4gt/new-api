package nvidia

import (
	"bytes"
	bytes2 "bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"golang.org/x/image/webp"
)

const (
	nvDinoV2InferURL = "https://ai.api.nvidia.com/v1/cv/nvidia/nv-dinov2"
	// NVIDIA official examples branch by base64 string length (< 200_000 inline; otherwise assets API).
	nvDinoV2InlineImageMaxBase64Chars  int64 = 200_000
	nvDinoV2DefaultImageMime                 = "image/jpeg"
	nvDinoV2InputAssetReferencesHeader       = "NVCF-INPUT-ASSET-REFERENCES"
	nvDinoV2FunctionAssetIDsHeader           = "NVCF-FUNCTION-ASSET-IDS"
)

var (
	nvDinoV2NvcfAssetsURL = "https://api.nvcf.nvidia.com/v2/nvcf/assets"

	buildNVDinoV2RequestAndRuntimeHeaders = buildNVDinoV2RequestAndRuntimeHeadersImpl
	createNvcfAsset                       = createNvcfAssetImpl
)

type nvDinoV2Request struct {
	Messages []nvDinoV2Message `json:"messages"`
}

type nvDinoV2Message struct {
	Content nvDinoV2Content `json:"content"`
}

type nvDinoV2Content struct {
	Type     string             `json:"type"`
	ImageURL nvDinoV2ImageInput `json:"image_url"`
}

type nvDinoV2ImageInput struct {
	URL string `json:"url"`
}

type nvDinoV2Response struct {
	Metadata []nvDinoV2Metadata `json:"metadata"`
	Created  int64              `json:"created"`
	Model    string             `json:"model"`
	Object   string             `json:"object"`
	Usage    nvDinoV2Usage      `json:"usage"`
}

type nvDinoV2Metadata struct {
	ID        string    `json:"id"`
	Embedding []float64 `json:"embedding"`
	FrameNum  int       `json:"frame_num"`
}

type nvDinoV2Usage struct {
	InferenceResponseTime int64 `json:"inference_response_time"`
}

type nvcfCreateAssetRequest struct {
	ContentType string `json:"contentType"`
	Description string `json:"description"`
}

type nvcfCreateAssetResponse struct {
	UploadURL string `json:"uploadUrl"`
	AssetID   string `json:"assetId"`
}

func buildNVDinoV2RequestAndRuntimeHeadersImpl(c *gin.Context, info *relaycommon.RelayInfo, input any) (*nvDinoV2Request, map[string]any, error) {
	imageRaw, err := parseSingleImageInput(input)
	if err != nil {
		return nil, nil, err
	}

	source := parseImageSource(imageRaw)
	cachedData, err := service.LoadFileSource(c, source, "nvidia_nv_dinov2_input")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load image input: %w", err)
	}

	base64Data, err := cachedData.GetBase64Data()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read image content: %w", err)
	}

	base64Data, mimeType, _, err := normalizeNVDinoV2ImagePayload(base64Data, cachedData)
	if err != nil {
		return nil, nil, err
	}
	if mimeType == "" {
		mimeType = nvDinoV2DefaultImageMime
	}
	base64Size := int64(len(base64Data))

	if base64Size < nvDinoV2InlineImageMaxBase64Chars {
		payload := &nvDinoV2Request{
			Messages: []nvDinoV2Message{
				{
					Content: nvDinoV2Content{
						Type: "image_url",
						ImageURL: nvDinoV2ImageInput{
							URL: fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data),
						},
					},
				},
			},
		}
		return payload, nil, nil
	}

	assetID, err := uploadNvcfAsset(c, info, base64Data, mimeType, "Input Image")
	if err != nil {
		return nil, nil, err
	}

	// NVIDIA official snippets document that large images should be uploaded as assets,
	// then referenced only via NVCF headers; body keeps empty messages.
	payload := &nvDinoV2Request{
		Messages: make([]nvDinoV2Message, 0),
	}

	runtimeHeaders := map[string]any{
		strings.ToLower(nvDinoV2InputAssetReferencesHeader): assetID,
		// Keep this optional compatibility header because NVIDIA examples include it.
		strings.ToLower(nvDinoV2FunctionAssetIDsHeader): assetID,
	}
	return payload, runtimeHeaders, nil
}

func parseSingleImageInput(input any) (string, error) {
	switch v := input.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return "", fmt.Errorf("input is empty")
		}
		return v, nil
	case []any:
		if len(v) != 1 {
			return "", fmt.Errorf("nv-dinov2 requires a single image input")
		}
		s, ok := v[0].(string)
		if !ok || strings.TrimSpace(s) == "" {
			return "", fmt.Errorf("nv-dinov2 input must be a non-empty string")
		}
		return s, nil
	default:
		return "", fmt.Errorf("nv-dinov2 input must be string or single-element string array")
	}
}

func parseImageSource(raw string) *types.FileSource {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return types.NewURLFileSource(trimmed)
	}
	return types.NewBase64FileSource(trimmed, "")
}

func normalizeImageMimeType(mimeType string) string {
	mimeType = strings.TrimSpace(strings.ToLower(mimeType))
	switch mimeType {
	case "image/jpg":
		return "image/jpeg"
	case "image/jpeg", "image/png":
		return mimeType
	case "":
		return ""
	default:
		return nvDinoV2DefaultImageMime
	}
}

func normalizeNVDinoV2ImagePayload(base64Data string, cachedData *types.CachedFileData) (string, string, int64, error) {
	if cachedData == nil {
		return base64Data, nvDinoV2DefaultImageMime, 0, nil
	}

	binaryData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", "", 0, fmt.Errorf("failed to decode base64 image: %w", err)
	}

	detectedMimeType := detectMimeTypeFromImageFormat(cachedData.ImageFormat)
	if detectedMimeType == "" {
		detectedMimeType = detectMimeTypeFromBinary(binaryData)
	}
	if detectedMimeType != "" {
		if detectedMimeType == "image/jpeg" || detectedMimeType == "image/png" {
			return base64Data, detectedMimeType, int64(len(binaryData)), nil
		}

		// Convert unsupported but decodable formats (e.g. webp) to jpeg.
		convertedBase64, convertedSize, convErr := transcodeImageToJPEG(binaryData)
		if convErr == nil {
			return convertedBase64, "image/jpeg", convertedSize, nil
		}
	}

	mimeType := normalizeImageMimeType(cachedData.MimeType)
	if mimeType == "" {
		mimeType = nvDinoV2DefaultImageMime
	}
	return base64Data, mimeType, int64(len(binaryData)), nil
}

func detectMimeTypeFromImageFormat(format string) string {
	switch strings.TrimSpace(strings.ToLower(format)) {
	case "jpeg", "jpg":
		return "image/jpeg"
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	case "gif":
		return "image/gif"
	case "bmp":
		return "image/bmp"
	case "tiff":
		return "image/tiff"
	default:
		return ""
	}
}

func detectMimeTypeFromBinary(binaryData []byte) string {
	if len(binaryData) == 0 {
		return ""
	}
	mimeType := strings.TrimSpace(strings.ToLower(http.DetectContentType(binaryData)))
	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	switch mimeType {
	case "image/jpg":
		return "image/jpeg"
	case "image/jpeg", "image/png", "image/webp", "image/gif", "image/bmp", "image/tiff":
		return mimeType
	default:
		return ""
	}
}

func transcodeImageToJPEG(binaryData []byte) (string, int64, error) {
	img, _, err := image.Decode(bytes.NewReader(binaryData))
	if err != nil {
		decodedWebP, webpErr := webp.Decode(bytes.NewReader(binaryData))
		if webpErr != nil {
			return "", 0, err
		}
		img = decodedWebP
	}
	var buf bytes.Buffer
	if err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		return "", 0, err
	}
	out := buf.Bytes()
	return base64.StdEncoding.EncodeToString(out), int64(len(out)), nil
}

func uploadNvcfAsset(c *gin.Context, info *relaycommon.RelayInfo, base64Data string, mimeType string, description string) (string, error) {
	binaryData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 image: %w", err)
	}

	assetResp, err := createNvcfAsset(c, info, mimeType, description)
	if err != nil {
		return "", err
	}

	if err = putNvcfAssetBinary(c, info, assetResp.UploadURL, mimeType, description, binaryData); err != nil {
		return "", err
	}

	return assetResp.AssetID, nil
}

func createNvcfAssetImpl(c *gin.Context, info *relaycommon.RelayInfo, mimeType string, description string) (*nvcfCreateAssetResponse, error) {
	payload := nvcfCreateAssetRequest{
		ContentType: mimeType,
		Description: description,
	}
	jsonData, err := common.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode NVCF create asset payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, nvDinoV2NvcfAssetsURL, bytes2.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to build NVCF create asset request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+info.ApiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client, err := service.GetHttpClientWithProxy(info.ChannelSetting.Proxy)
	if err != nil {
		return nil, fmt.Errorf("failed to create http client: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create NVCF asset: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read NVCF create asset response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("NVCF create asset failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	assetResp := &nvcfCreateAssetResponse{}
	if err = common.Unmarshal(bodyBytes, assetResp); err != nil {
		return nil, fmt.Errorf("failed to decode NVCF create asset response: %w", err)
	}
	if strings.TrimSpace(assetResp.UploadURL) == "" || strings.TrimSpace(assetResp.AssetID) == "" {
		return nil, fmt.Errorf("NVCF create asset response missing uploadUrl or assetId")
	}
	return assetResp, nil
}

func putNvcfAssetBinary(c *gin.Context, info *relaycommon.RelayInfo, uploadURL string, mimeType string, description string, data []byte) error {
	req, err := http.NewRequest(http.MethodPut, uploadURL, bytes2.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to build NVCF asset upload request: %w", err)
	}
	// NVIDIA official sample sets these S3 metadata headers during upload.
	req.Header.Set("x-amz-meta-nvcf-asset-description", description)
	req.Header.Set("content-type", mimeType)

	client, err := service.GetHttpClientWithProxy(info.ChannelSetting.Proxy)
	if err != nil {
		return fmt.Errorf("failed to create http client: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload NVCF asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("NVCF asset upload failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	return nil
}

func convertNVDinoV2ResponseToOpenAI(resp *nvDinoV2Response, model string, estimatePromptTokens int) *dto.OpenAIEmbeddingResponse {
	result := &dto.OpenAIEmbeddingResponse{
		Object: "list",
		Model:  model,
		Data:   make([]dto.OpenAIEmbeddingResponseItem, 0),
		Usage: dto.Usage{
			PromptTokens: estimatePromptTokens,
			TotalTokens:  estimatePromptTokens,
		},
	}

	for i, item := range resp.Metadata {
		result.Data = append(result.Data, dto.OpenAIEmbeddingResponseItem{
			Object:    "embedding",
			Index:     i,
			Embedding: item.Embedding,
		})
	}

	return result
}

func nvDinoV2EmbeddingHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var nvResp nvDinoV2Response
	if err = common.Unmarshal(responseBody, &nvResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	estimatePromptTokens := info.GetEstimatePromptTokens()
	converted := convertNVDinoV2ResponseToOpenAI(&nvResp, info.UpstreamModelName, estimatePromptTokens)

	out, err := common.Marshal(converted)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	service.IOCopyBytesGracefully(c, resp, out)
	return &converted.Usage, nil
}
