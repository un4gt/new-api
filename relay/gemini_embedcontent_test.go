package relay

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/require"
)

func TestCountPDFPagesFromBase64_TypePageHeuristic(t *testing.T) {
	t.Parallel()

	// Not a full valid PDF; sufficient for the heuristic which scans for "/Type /Page".
	fakePDF := []byte("%PDF-1.4\n" + strings.Repeat("<< /Type /Page >>\n", 7))
	b64 := base64.StdEncoding.EncodeToString(fakePDF)

	pages, determined, err := countPDFPagesFromBase64(b64, 6)
	require.NoError(t, err)
	require.True(t, determined)
	require.Equal(t, 7, pages)
}

func TestCountPDFPagesFromBase64_PagesCountHeuristic(t *testing.T) {
	t.Parallel()

	// Prefer /Type /Pages + /Count when present.
	fakePDF := []byte("%PDF-1.4\n<< /Type /Pages /Count 9 >>\n")
	b64 := base64.StdEncoding.EncodeToString(fakePDF)

	pages, determined, err := countPDFPagesFromBase64(b64, 6)
	require.NoError(t, err)
	require.True(t, determined)
	require.Equal(t, 9, pages)
}

func TestBuildEmbedding2EmbedContentUpstreamRequest_GeminiDoesNotSendModelField(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:           constant.APITypeGemini,
			UpstreamModelName: "gemini-embedding-2-preview",
		},
	}

	taskType := "RETRIEVAL_DOCUMENT"
	title := "doc"
	ocr := true
	dims := 768
	req := &dto.EmbedContentRequest{
		Content: dto.GeminiChatContent{Parts: []dto.GeminiPart{{Text: "hello"}}},
		EmbedContentConfig: &dto.EmbedContentConfig{
			TaskType:             &taskType,
			Title:                &title,
			DocumentOcr:          &ocr,
			OutputDimensionality: &dims,
		},
	}

	upstreamReq, err := buildEmbedding2EmbedContentUpstreamRequest(info, req)
	require.NoError(t, err)

	geminiReq, ok := upstreamReq.(*dto.GeminiEmbeddingRequest)
	require.True(t, ok)
	require.Equal(t, "", geminiReq.Model)
	require.Equal(t, taskType, geminiReq.TaskType)
	require.Equal(t, title, geminiReq.Title)
	require.Equal(t, dims, geminiReq.OutputDimensionality)

	bytes, err := common.Marshal(geminiReq)
	require.NoError(t, err)

	var body map[string]any
	err = common.Unmarshal(bytes, &body)
	require.NoError(t, err)
	require.NotContains(t, body, "model")
	require.Contains(t, body, "content")
	require.Contains(t, body, "taskType")
	require.Contains(t, body, "title")
	require.Contains(t, body, "outputDimensionality")
}

func TestBuildEmbedding2EmbedContentUpstreamRequest_Gemini001KeepsTopLevelConfig(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiType:           constant.APITypeGemini,
			UpstreamModelName: "gemini-embedding-001",
		},
	}

	req := &dto.EmbedContentRequest{
		Content: dto.GeminiChatContent{Parts: []dto.GeminiPart{{Text: "hello 001"}}},
	}

	upstreamReq, err := buildEmbedding2EmbedContentUpstreamRequest(info, req)
	require.NoError(t, err)

	geminiReq, ok := upstreamReq.(*dto.GeminiEmbeddingRequest)
	require.True(t, ok)
	require.Equal(t, "", geminiReq.Model)
	require.Equal(t, "hello 001", geminiReq.Content.Parts[0].Text)

	bytes, err := common.Marshal(geminiReq)
	require.NoError(t, err)

	var body map[string]any
	err = common.Unmarshal(bytes, &body)
	require.NoError(t, err)
	require.NotContains(t, body, "model")
	require.Contains(t, body, "content")
}
