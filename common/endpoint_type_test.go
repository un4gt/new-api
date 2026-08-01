package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

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

			got := GetEndpointTypesByChannelType(constant.ChannelTypeMoark, tc.modelName)
			if len(got) != len(tc.expected) {
				t.Fatalf("GetEndpointTypesByChannelType(%q) len = %d, want %d", tc.modelName, len(got), len(tc.expected))
			}
			for i := range tc.expected {
				if got[i] != tc.expected[i] {
					t.Fatalf("GetEndpointTypesByChannelType(%q)[%d] = %q, want %q", tc.modelName, i, got[i], tc.expected[i])
				}
			}
		})
	}
}

func TestGetEndpointTypesByChannelTypeOpenRouterGeminiEmbedding2(t *testing.T) {
	t.Parallel()

	got := GetEndpointTypesByChannelType(constant.ChannelTypeOpenRouter, constant.OpenRouterGeminiEmbedding2PreviewModel)
	expected := []constant.EndpointType{constant.EndpointTypeEmbeddings}
	if len(got) != len(expected) {
		t.Fatalf("GetEndpointTypesByChannelType(OpenRouter, %q) len = %d, want %d", constant.OpenRouterGeminiEmbedding2PreviewModel, len(got), len(expected))
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("GetEndpointTypesByChannelType(OpenRouter, %q)[%d] = %q, want %q", constant.OpenRouterGeminiEmbedding2PreviewModel, i, got[i], expected[i])
		}
	}
}

func TestGetEndpointTypesByChannelTypeOpenRouterDefault(t *testing.T) {
	t.Parallel()

	got := GetEndpointTypesByChannelType(constant.ChannelTypeOpenRouter, "cohere/rerank-v3.5")
	expected := []constant.EndpointType{constant.EndpointTypeOpenAI}
	if len(got) != len(expected) {
		t.Fatalf("GetEndpointTypesByChannelType(OpenRouter, cohere/rerank-v3.5) len = %d, want %d", len(got), len(expected))
	}
	for i := range expected {
		if got[i] != expected[i] {
			t.Fatalf("GetEndpointTypesByChannelType(OpenRouter, cohere/rerank-v3.5)[%d] = %q, want %q", i, got[i], expected[i])
		}
	}
}

func TestGetEndpointTypesByChannelTypeVoyageAIByMongoDB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model    string
		expected constant.EndpointType
	}{
		{model: "voyage-4-large", expected: constant.EndpointTypeEmbeddings},
		{model: "voyage-multimodal-3.5", expected: constant.EndpointTypeEmbeddings},
		{model: "rerank-2.5", expected: constant.EndpointTypeJinaRerank},
	}
	for _, test := range tests {
		test := test
		t.Run(test.model, func(t *testing.T) {
			t.Parallel()
			got := GetEndpointTypesByChannelType(constant.ChannelTypeVoyageAIByMongoDB, test.model)
			if len(got) != 1 || got[0] != test.expected {
				t.Fatalf("unexpected endpoint types for %s: %v", test.model, got)
			}
		})
	}
}
