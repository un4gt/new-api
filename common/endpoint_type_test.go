package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func requireEndpointTypes(t *testing.T, channelType int, modelName string, expected []constant.EndpointType) {
	t.Helper()

	got := GetEndpointTypesByChannelType(channelType, modelName)
	if len(got) != len(expected) {
		t.Fatalf("GetEndpointTypesByChannelType(%d, %q) len = %d, want %d", channelType, modelName, len(got), len(expected))
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("GetEndpointTypesByChannelType(%d, %q)[%d] = %q, want %q", channelType, modelName, i, got[i], expected[i])
		}
	}
}

func TestGetEndpointTypesByChannelTypeMoarkPreciseModels(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		modelName string
		expected  []constant.EndpointType
	}{
		{
			name:      "multimodal reranker uses rerank multimodal endpoint",
			modelName: "Qwen3-VL-Reranker-2B",
			expected:  []constant.EndpointType{constant.EndpointTypeRerankMultimodal},
		},
		{
			name:      "jina reranker m0 uses rerank multimodal endpoint",
			modelName: "jina-reranker-m0",
			expected:  []constant.EndpointType{constant.EndpointTypeRerankMultimodal},
		},
		{
			name:      "text reranker uses rerank endpoint",
			modelName: "bge-reranker-v2-m3",
			expected:  []constant.EndpointType{constant.EndpointTypeJinaRerank},
		},
		{
			name:      "vl embedding uses embeddings endpoint",
			modelName: "Qwen3-VL-Embedding-2B",
			expected:  []constant.EndpointType{constant.EndpointTypeEmbeddings},
		},
		{
			name:      "jina multimodal embedding uses embeddings endpoint",
			modelName: "jina-embeddings-v4",
			expected:  []constant.EndpointType{constant.EndpointTypeEmbeddings},
		},
		{
			name:      "code embedding uses embeddings endpoint",
			modelName: "nomic-embed-code",
			expected:  []constant.EndpointType{constant.EndpointTypeEmbeddings},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireEndpointTypes(t, constant.ChannelTypeMoark, tc.modelName, tc.expected)
		})
	}
}

func TestGetEndpointTypesByChannelTypeCohere(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		modelName string
		expected  []constant.EndpointType
	}{
		{
			name:      "text embedding uses embeddings endpoint",
			modelName: "embed-v4.0",
			expected:  []constant.EndpointType{constant.EndpointTypeEmbeddings},
		},
		{
			name:      "image embedding uses embeddings endpoint",
			modelName: "embed-english-v3.0-image",
			expected:  []constant.EndpointType{constant.EndpointTypeEmbeddings},
		},
		{
			name:      "reranker uses rerank endpoint",
			modelName: "rerank-v4.0-pro",
			expected:  []constant.EndpointType{constant.EndpointTypeJinaRerank},
		},
		{
			name:      "unknown cohere model keeps supported minimal endpoints",
			modelName: "custom-cohere-model",
			expected:  []constant.EndpointType{constant.EndpointTypeEmbeddings, constant.EndpointTypeJinaRerank},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			requireEndpointTypes(t, constant.ChannelTypeCohere, tc.modelName, tc.expected)
		})
	}
}

func TestGetEndpointTypesByChannelTypeOpenRouterGeminiEmbedding2(t *testing.T) {
	t.Parallel()

	expected := []constant.EndpointType{constant.EndpointTypeEmbeddings}
	requireEndpointTypes(t, constant.ChannelTypeOpenRouter, constant.OpenRouterGeminiEmbedding2PreviewModel, expected)
}

func TestGetEndpointTypesByChannelTypeOpenRouterDefault(t *testing.T) {
	t.Parallel()

	expected := []constant.EndpointType{constant.EndpointTypeOpenAI}
	requireEndpointTypes(t, constant.ChannelTypeOpenRouter, "cohere/rerank-v3.5", expected)
}
