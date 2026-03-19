package relay

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	embedding2MaxInputTokens          = 8192
	embedding2MaxOutputDimensionality = 3072

	embedding2MaxImagesPerPrompt   = 6
	embedding2MaxDocsPerPrompt     = 1
	embedding2MaxDocPagesPerFile   = 6
	embedding2MaxVideosPerPrompt   = 1
	embedding2MaxAudiosPerPrompt   = 1
	embedding2MaxAudioSeconds      = 80
	embedding2MaxVideoSecondsAudio = 80
	embedding2MaxVideoSecondsNoAud = 120
)

type embedContentMediaCounts struct {
	Images int
	Docs   int
	Videos int
	Audios int
}

func geminiEmbedContentHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	req := &dto.EmbedContentRequest{}
	if err := common.UnmarshalBodyReusable(c, req); err != nil {
		return types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if len(req.Content.Parts) == 0 {
		return types.NewErrorWithStatusCode(fmt.Errorf("content.parts is required"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	// Apply model mapping.
	if err := helper.ModelMappedHelper(c, info, req); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	// This endpoint is reserved for gemini-embedding-2-preview multimodal embeddings.
	if info.UpstreamModelName != "gemini-embedding-2-preview" {
		return types.NewErrorWithStatusCode(
			fmt.Errorf("model %q is not supported on :embedContent; use /v1/embeddings for text embeddings or switch to gemini-embedding-2-preview", info.UpstreamModelName),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	counts, newAPIError := validateAndNormalizeEmbedding2EmbedContentRequest(c, req)
	if newAPIError != nil {
		return newAPIError
	}

	logger.LogDebug(c, fmt.Sprintf("Gemini embedContent request prepared: model=%s parts=%d images=%d docs=%d videos=%d audios=%d max_tokens=%d",
		info.UpstreamModelName,
		len(req.Content.Parts),
		counts.Images,
		counts.Docs,
		counts.Videos,
		counts.Audios,
		embedding2MaxInputTokens,
	))

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	jsonData, err := common.Marshal(req)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}
	// apply param override
	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			return newAPIErrorFromParamOverride(err)
		}
	}

	resp, err := adaptor.DoRequest(c, info, bytes.NewReader(jsonData))
	if err != nil {
		logger.LogError(c, "Do gemini embedContent request failed: "+err.Error())
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")
	if resp != nil {
		httpResp := resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, openaiErr := adaptor.DoResponse(c, resp.(*http.Response), info)
	if openaiErr != nil {
		service.ResetStatusCode(openaiErr, statusCodeMappingStr)
		return openaiErr
	}

	postConsumeQuota(c, info, usage.(*dto.Usage))
	return nil
}

func validateAndNormalizeEmbedding2EmbedContentRequest(c *gin.Context, req *dto.EmbedContentRequest) (*embedContentMediaCounts, *types.NewAPIError) {
	effectiveDims, newAPIError := validateEmbedding2OutputDimensionality(req)
	if newAPIError != nil {
		return nil, newAPIError
	}
	if effectiveDims != nil && (*effectiveDims <= 0 || *effectiveDims > embedding2MaxOutputDimensionality) {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("outputDimensionality must be between 1 and %d", embedding2MaxOutputDimensionality),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	maxImageMB := system_setting.GetEmbeddingLimits().Embedding2ImageMaxMB
	if maxImageMB <= 0 {
		maxImageMB = 20
	}
	maxImageBytes := int64(maxImageMB) << 20

	counts := &embedContentMediaCounts{}

	for i := range req.Content.Parts {
		part := &req.Content.Parts[i]

		// Reject tool/function related parts for embeddings.
		if part.FunctionCall != nil || part.FunctionResponse != nil || part.ExecutableCode != nil || part.CodeExecutionResult != nil {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("content.parts[%d] contains unsupported fields for embeddings", i),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}

		if part.InlineData == nil && part.FileData == nil && strings.TrimSpace(part.Text) == "" {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("content.parts[%d] is empty", i),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}

		if part.InlineData != nil {
			mimeType := normalizeMimeType(part.InlineData.MimeType)
			modality, err := embedding2ModalityForMime(mimeType)
			if err != nil {
				return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			source := types.NewBase64FileSource(part.InlineData.Data, mimeType)
			cachedData, err := service.LoadFileSource(c, source, "embedding2_inline_data")
			if err != nil {
				return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			base64Data, err := cachedData.GetBase64Data()
			if err != nil {
				return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			part.InlineData.MimeType = mimeType
			part.InlineData.Data = base64Data

			if modality == "image" && cachedData.Size > maxImageBytes {
				return nil, types.NewErrorWithStatusCode(
					fmt.Errorf("image size exceeds limit: %dMB", maxImageMB),
					types.ErrorCodeInvalidRequest,
					http.StatusBadRequest,
					types.ErrOptionWithSkipRetry(),
				)
			}

			if newAPIError := enforceEmbedding2MediaConstraints(c, modality, mimeType, base64Data, counts); newAPIError != nil {
				return nil, newAPIError
			}
			continue
		}

		if part.FileData != nil {
			mimeType := normalizeMimeType(part.FileData.MimeType)
			modality, err := embedding2ModalityForMime(mimeType)
			if err != nil {
				return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			fileURI := strings.TrimSpace(part.FileData.FileUri)
			if fileURI == "" {
				return nil, types.NewErrorWithStatusCode(
					fmt.Errorf("content.parts[%d].fileData.fileUri is required", i),
					types.ErrorCodeInvalidRequest,
					http.StatusBadRequest,
					types.ErrOptionWithSkipRetry(),
				)
			}
			if strings.HasPrefix(fileURI, "gs://") {
				return nil, types.NewErrorWithStatusCode(
					fmt.Errorf("Google Cloud Storage (gs://) is not supported for embedContent; use inline_data or http(s) file_uri"),
					types.ErrorCodeInvalidRequest,
					http.StatusBadRequest,
					types.ErrOptionWithSkipRetry(),
				)
			}
			if !strings.HasPrefix(fileURI, "http://") && !strings.HasPrefix(fileURI, "https://") {
				return nil, types.NewErrorWithStatusCode(
					fmt.Errorf("unsupported file_uri scheme: %s", fileURI),
					types.ErrorCodeInvalidRequest,
					http.StatusBadRequest,
					types.ErrOptionWithSkipRetry(),
				)
			}

			source := types.NewURLFileSource(fileURI)
			cachedData, err := service.LoadFileSource(c, source, "embedding2_file_data")
			if err != nil {
				return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			base64Data, err := cachedData.GetBase64Data()
			if err != nil {
				return nil, types.NewErrorWithStatusCode(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}

			part.InlineData = &dto.GeminiInlineData{
				MimeType: mimeType,
				Data:     base64Data,
			}
			part.FileData = nil

			if modality == "image" && cachedData.Size > maxImageBytes {
				return nil, types.NewErrorWithStatusCode(
					fmt.Errorf("image size exceeds limit: %dMB", maxImageMB),
					types.ErrorCodeInvalidRequest,
					http.StatusBadRequest,
					types.ErrOptionWithSkipRetry(),
				)
			}

			if newAPIError := enforceEmbedding2MediaConstraints(c, modality, mimeType, base64Data, counts); newAPIError != nil {
				return nil, newAPIError
			}
		}
	}

	if counts.Images > embedding2MaxImagesPerPrompt ||
		counts.Docs > embedding2MaxDocsPerPrompt ||
		counts.Videos > embedding2MaxVideosPerPrompt ||
		counts.Audios > embedding2MaxAudiosPerPrompt {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("embedContent media limits exceeded (images<=%d, docs<=%d, videos<=%d, audios<=%d)",
				embedding2MaxImagesPerPrompt, embedding2MaxDocsPerPrompt, embedding2MaxVideosPerPrompt, embedding2MaxAudiosPerPrompt),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	return counts, nil
}

func validateEmbedding2OutputDimensionality(req *dto.EmbedContentRequest) (*int, *types.NewAPIError) {
	var topLevel *int = req.OutputDimensionality
	var cfgLevel *int
	if req.EmbedContentConfig != nil {
		cfgLevel = req.EmbedContentConfig.OutputDimensionality
	}

	if topLevel != nil && cfgLevel != nil && *topLevel != *cfgLevel {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("outputDimensionality is inconsistent between top-level and embedContentConfig"),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	if cfgLevel != nil {
		return cfgLevel, nil
	}
	return topLevel, nil
}

func normalizeMimeType(mimeType string) string {
	mimeType = strings.TrimSpace(mimeType)
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}
	return strings.ToLower(mimeType)
}

func embedding2ModalityForMime(mimeType string) (string, error) {
	switch mimeType {
	case "image/png", "image/jpeg":
		return "image", nil
	case "application/pdf":
		return "doc", nil
	case "video/mpeg", "video/mp4":
		return "video", nil
	case "audio/mp3", "audio/wav":
		return "audio", nil
	case "":
		return "", fmt.Errorf("mimeType is required")
	default:
		return "", fmt.Errorf("unsupported mimeType for gemini-embedding-2-preview: %s", mimeType)
	}
}

func enforceEmbedding2MediaConstraints(c *gin.Context, modality string, mimeType string, base64Data string, counts *embedContentMediaCounts) *types.NewAPIError {
	if counts == nil {
		counts = &embedContentMediaCounts{}
	}

	switch modality {
	case "image":
		counts.Images++
		if counts.Images > embedding2MaxImagesPerPrompt {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("too many images in prompt (max %d)", embedding2MaxImagesPerPrompt),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
		return nil

	case "doc":
		counts.Docs++
		if counts.Docs > embedding2MaxDocsPerPrompt {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("too many documents in prompt (max %d)", embedding2MaxDocsPerPrompt),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}

		// Best-effort PDF page limit enforcement (docs say up to 6 pages).
		if mimeType == "application/pdf" && base64Data != "" {
			pages, determined, err := countPDFPagesFromBase64(base64Data, embedding2MaxDocPagesPerFile)
			if err != nil {
				return types.NewErrorWithStatusCode(
					fmt.Errorf("failed to parse PDF page count: %w", err),
					types.ErrorCodeInvalidRequest,
					http.StatusBadRequest,
					types.ErrOptionWithSkipRetry(),
				)
			}
			if determined && pages > embedding2MaxDocPagesPerFile {
				return types.NewErrorWithStatusCode(
					fmt.Errorf("pdf page count exceeds limit: %d (max %d)", pages, embedding2MaxDocPagesPerFile),
					types.ErrorCodeInvalidRequest,
					http.StatusBadRequest,
					types.ErrOptionWithSkipRetry(),
				)
			}
		}
		return nil

	case "audio":
		counts.Audios++
		if counts.Audios > embedding2MaxAudiosPerPrompt {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("too many audio files in prompt (max %d)", embedding2MaxAudiosPerPrompt),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}

		raw, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return types.NewErrorWithStatusCode(fmt.Errorf("invalid base64 audio data: %w", err), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		ext := ""
		switch mimeType {
		case "audio/mp3":
			ext = ".mp3"
		case "audio/wav":
			ext = ".wav"
		}
		duration, err := common.GetAudioDuration(c.Request.Context(), bytes.NewReader(raw), ext)
		if err != nil {
			return types.NewErrorWithStatusCode(fmt.Errorf("failed to parse audio duration: %w", err), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		if duration > embedding2MaxAudioSeconds {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("audio duration exceeds limit: %.2fs (max %ds)", duration, embedding2MaxAudioSeconds),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
		return nil

	case "video":
		counts.Videos++
		if counts.Videos > embedding2MaxVideosPerPrompt {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("too many videos in prompt (max %d)", embedding2MaxVideosPerPrompt),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}

		raw, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			return types.NewErrorWithStatusCode(fmt.Errorf("invalid base64 video data: %w", err), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}

		duration, hasAudio, err := common.GetVideoDuration(c.Request.Context(), bytes.NewReader(raw), mimeType)
		if err != nil {
			return types.NewErrorWithStatusCode(fmt.Errorf("failed to parse video duration: %w", err), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		limit := embedding2MaxVideoSecondsNoAud
		if hasAudio {
			limit = embedding2MaxVideoSecondsAudio
		}
		if duration > float64(limit) {
			return types.NewErrorWithStatusCode(
				fmt.Errorf("video duration exceeds limit: %.2fs (max %ds, hasAudio=%t)", duration, limit, hasAudio),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
		return nil
	}

	return types.NewErrorWithStatusCode(
		fmt.Errorf("unsupported modality: %s", modality),
		types.ErrorCodeInvalidRequest,
		http.StatusBadRequest,
		types.ErrOptionWithSkipRetry(),
	)
}

func countPDFPagesFromBase64(base64Data string, maxPages int) (pages int, determined bool, err error) {
	if maxPages <= 0 {
		maxPages = 1
	}

	const (
		windowTailBytes  = 2048
		chunkBytes       = 32 * 1024
		keywordType      = "/Type"
		keywordPage      = "Page"
		keywordPages     = "Pages"
		keywordCount     = "/Count"
		countSearchBytes = 512
	)

	dec := base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64Data))
	buf := make([]byte, chunkBytes)
	tail := make([]byte, 0, windowTailBytes)
	var totalDecoded int64

	var pagesByType int
	var pagesByCount int
	var lastPageTypeAbsStart int64 = -1

	for {
		n, readErr := dec.Read(buf)
		if n > 0 {
			chunk := buf[:n]

			window := make([]byte, 0, len(tail)+len(chunk))
			window = append(window, tail...)
			window = append(window, chunk...)

			windowStartAbs := totalDecoded - int64(len(tail))

			// Scan for /Type /Page and /Type /Pages + /Count.
			i := 0
			for {
				idx := bytes.Index(window[i:], []byte(keywordType))
				if idx < 0 {
					break
				}
				pos := i + idx

				j := pos + len(keywordType)
				for j < len(window) && isPDFWhitespace(window[j]) {
					j++
				}
				if j >= len(window) || window[j] != '/' {
					i = pos + len(keywordType)
					continue
				}
				j++

				// /Type /Pages => try to find /Count nearby.
				if hasPDFTokenPrefix(window, j, keywordPages) {
					searchStart := pos - countSearchBytes
					if searchStart < 0 {
						searchStart = 0
					}
					searchEnd := j + len(keywordPages) + countSearchBytes
					if searchEnd > len(window) {
						searchEnd = len(window)
					}
					if searchStart < searchEnd {
						if v := maxPDFCountValue(window[searchStart:searchEnd], keywordCount); v > pagesByCount {
							pagesByCount = v
							if pagesByCount > maxPages {
								return pagesByCount, true, nil
							}
						}
					}
					i = pos + len(keywordType)
					continue
				}

				// /Type /Page (exclude /Pages)
				if hasPDFTokenPrefix(window, j, keywordPage) && !hasPDFTokenPrefix(window, j, keywordPages) {
					absStart := windowStartAbs + int64(pos)
					if absStart > lastPageTypeAbsStart {
						pagesByType++
						lastPageTypeAbsStart = absStart
						if pagesByType > maxPages {
							return pagesByType, true, nil
						}
					}
				}
				i = pos + len(keywordType)
			}

			if len(window) > windowTailBytes {
				tail = append(tail[:0], window[len(window)-windowTailBytes:]...)
			} else {
				tail = append(tail[:0], window...)
			}
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return 0, false, readErr
		}

		totalDecoded += int64(n)
	}

	if pagesByCount > 0 {
		return pagesByCount, true, nil
	}
	if pagesByType > 0 {
		return pagesByType, true, nil
	}
	return 0, false, nil
}

func isPDFWhitespace(b byte) bool {
	switch b {
	case 0x00, 0x09, 0x0A, 0x0C, 0x0D, 0x20:
		return true
	default:
		return false
	}
}

func hasPDFTokenPrefix(b []byte, start int, token string) bool {
	if start < 0 || start+len(token) > len(b) {
		return false
	}
	for i := 0; i < len(token); i++ {
		if b[start+i] != token[i] {
			return false
		}
	}
	// Ensure word boundary: "Page" should not match "Pages", etc.
	next := start + len(token)
	if next >= len(b) {
		return true
	}
	switch b[next] {
	case 0x00, 0x09, 0x0A, 0x0C, 0x0D, 0x20, '/', '>', '<', '[', ']', '(', ')':
		return true
	default:
		// Letter/digit/etc => token continues.
		return false
	}
}

func maxPDFCountValue(b []byte, keyword string) int {
	if len(b) == 0 {
		return 0
	}
	maxVal := 0
	i := 0
	for {
		idx := bytes.Index(b[i:], []byte(keyword))
		if idx < 0 {
			break
		}
		pos := i + idx + len(keyword)
		for pos < len(b) && isPDFWhitespace(b[pos]) {
			pos++
		}
		val := 0
		digits := 0
		for pos < len(b) {
			ch := b[pos]
			if ch < '0' || ch > '9' {
				break
			}
			val = val*10 + int(ch-'0')
			pos++
			digits++
			if digits > 9 {
				break
			}
		}
		if digits > 0 && val > maxVal {
			maxVal = val
		}
		i = i + idx + len(keyword)
	}
	return maxVal
}
