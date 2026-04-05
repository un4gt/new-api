package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func TestBuildTestRequestAutoDetectsEmbedModelsAsEmbedding(t *testing.T) {
	t.Parallel()

	req := buildTestRequest(
		"nvidia/llama-nemotron-embed-1b-v2",
		"",
		&model.Channel{Type: constant.ChannelTypeNvidia},
		false,
	)
	if _, ok := req.(*dto.EmbeddingRequest); !ok {
		t.Fatalf("expected *dto.EmbeddingRequest, got %T", req)
	}
}

func TestBuildTestRequestAutoDetectsMokaAIAsEmbedding(t *testing.T) {
	t.Parallel()

	req := buildTestRequest(
		"some-model-without-embed",
		"",
		&model.Channel{Type: constant.ChannelTypeMokaAI},
		false,
	)
	if _, ok := req.(*dto.EmbeddingRequest); !ok {
		t.Fatalf("expected *dto.EmbeddingRequest, got %T", req)
	}
}

func TestDetectTestRequestPathAutoDetectsMoarkMultimodalRerank(t *testing.T) {
	t.Parallel()

	path := detectTestRequestPath(
		&model.Channel{Type: constant.ChannelTypeMoark},
		"Qwen3-VL-Reranker-2B",
		"",
	)
	if path != "/v1/rerank/multimodal" {
		t.Fatalf("expected /v1/rerank/multimodal, got %s", path)
	}
}

func TestBuildTestRequestAutoDetectsMoarkMultimodalEmbedding(t *testing.T) {
	t.Parallel()

	req := buildTestRequest(
		"Qwen3-VL-Embedding-2B",
		"",
		&model.Channel{Type: constant.ChannelTypeMoark},
		false,
	)

	embeddingReq, ok := req.(*dto.EmbeddingRequest)
	if !ok {
		t.Fatalf("expected *dto.EmbeddingRequest, got %T", req)
	}
	input, ok := embeddingReq.Input.([]any)
	if !ok {
		t.Fatalf("expected []any input, got %T", embeddingReq.Input)
	}
	if len(input) != 3 {
		t.Fatalf("expected 3 multimodal embedding items, got %d", len(input))
	}
	lastItem, ok := input[2].(map[string]any)
	if !ok {
		t.Fatalf("expected image item to be map[string]any, got %T", input[2])
	}
	if _, ok := lastItem["image"].(string); !ok {
		t.Fatalf("expected image field in last embedding item, got %+v", lastItem)
	}
}

func TestBuildTestRequestAutoDetectsMoarkMultimodalRerank(t *testing.T) {
	t.Parallel()

	req := buildTestRequest(
		"jina-reranker-m0",
		"",
		&model.Channel{Type: constant.ChannelTypeMoark},
		false,
	)

	rerankReq, ok := req.(*dto.RerankMultimodalRequest)
	if !ok {
		t.Fatalf("expected *dto.RerankMultimodalRequest, got %T", req)
	}
	if rerankReq.Query.Image == nil || *rerankReq.Query.Image == "" {
		t.Fatalf("expected multimodal rerank query image to be populated")
	}
	if len(rerankReq.Documents) != 3 {
		t.Fatalf("expected 3 multimodal rerank documents, got %d", len(rerankReq.Documents))
	}
}

func TestBuildTestRequestAutoDetectsMoarkCodeEmbedding(t *testing.T) {
	t.Parallel()

	req := buildTestRequest(
		"nomic-embed-code",
		"",
		&model.Channel{Type: constant.ChannelTypeMoark},
		false,
	)

	embeddingReq, ok := req.(*dto.EmbeddingRequest)
	if !ok {
		t.Fatalf("expected *dto.EmbeddingRequest, got %T", req)
	}
	input, ok := embeddingReq.Input.(string)
	if !ok {
		t.Fatalf("expected string input, got %T", embeddingReq.Input)
	}
	if input == "" {
		t.Fatalf("expected non-empty string input for nomic-embed-code")
	}
}
