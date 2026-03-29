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
