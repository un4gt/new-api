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
