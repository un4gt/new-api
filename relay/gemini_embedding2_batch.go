package relay

import (
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// validateAndNormalizeEmbedding2BatchEmbeddingRequest applies the same gateway-side
// validations/normalizations as :embedContent (file handling, modality limits, etc.)
// but for :batchEmbedContents payloads.
//
// Note: we intentionally do not attempt to "merge" multiple requests into a single
// embedContent call; we simply validate and normalize each request independently.
func validateAndNormalizeEmbedding2BatchEmbeddingRequest(c *gin.Context, req *dto.GeminiBatchEmbeddingRequest) (*embedContentMediaCounts, *types.NewAPIError) {
	if req == nil {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("request is nil"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	if len(req.Requests) == 0 {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("requests is required"), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	total := &embedContentMediaCounts{}
	for i, r := range req.Requests {
		if r == nil {
			return nil, types.NewErrorWithStatusCode(fmt.Errorf("requests[%d] is null", i), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}

		// Convert GeminiEmbeddingRequest to the EmbedContentRequest schema so we can
		// reuse the existing embedding-2 validations/normalizations.
		embedReq := &dto.EmbedContentRequest{
			Content: r.Content,
		}
		if r.TaskType != "" || r.Title != "" || r.OutputDimensionality > 0 {
			embedReq.EmbedContentConfig = &dto.EmbedContentConfig{}
			if r.TaskType != "" {
				taskType := r.TaskType
				embedReq.EmbedContentConfig.TaskType = &taskType
			}
			if r.Title != "" {
				title := r.Title
				embedReq.EmbedContentConfig.Title = &title
			}
			if r.OutputDimensionality > 0 {
				dims := r.OutputDimensionality
				embedReq.EmbedContentConfig.OutputDimensionality = &dims
			}
		}

		counts, newAPIError := validateAndNormalizeEmbedding2EmbedContentRequest(c, embedReq)
		if newAPIError != nil {
			// Add request index context for easier debugging.
			statusCode := newAPIError.StatusCode
			if statusCode == 0 {
				statusCode = http.StatusBadRequest
			}
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("requests[%d]: %w", i, newAPIError.Err),
				newAPIError.GetErrorCode(),
				statusCode,
				types.ErrOptionWithSkipRetry(),
			)
		}

		// Write back normalized content (e.g. fileData -> inlineData, normalized mime/data).
		r.Content = embedReq.Content

		total.Images += counts.Images
		total.Docs += counts.Docs
		total.Videos += counts.Videos
		total.Audios += counts.Audios
	}

	return total, nil
}
