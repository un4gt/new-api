package cloudflare

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloudflareEmbeddingModelSpecs(t *testing.T) {
	t.Parallel()

	expected := []modelSpec{
		{ID: "cf/plamo-embedding-1b", UpstreamID: "@cf/pfnet/plamo-embedding-1b", SupportsBatch: false},
		{ID: "cf/embeddinggemma-300m", UpstreamID: "@cf/google/embeddinggemma-300m", SupportsBatch: false},
		{ID: "cf/qwen-embedding-0.6b", UpstreamID: "@cf/qwen/qwen3-embedding-0.6b", SupportsBatch: false},
		{ID: "cf/bge-m3", UpstreamID: "@cf/baai/bge-m3", SupportsBatch: true},
		{ID: "cf/bge-large-en-v1.5", UpstreamID: "@cf/baai/bge-large-en-v1.5", SupportsBatch: true},
		{ID: "cf/bge-small-en-v1.5", UpstreamID: "@cf/baai/bge-small-en-v1.5", SupportsBatch: true},
		{ID: "cf/bge-base-en-v1.5", UpstreamID: "@cf/baai/bge-base-en-v1.5", SupportsBatch: true},
	}

	require.Equal(t, expected, modelSpecs)
	require.Len(t, ModelList, len(expected))
	for index, spec := range expected {
		require.Equal(t, spec.ID, ModelList[index])
	}
	require.NotContains(t, ModelList, "@cf/meta/llama-3.1-8b-instruct")
	require.NotContains(t, ModelList, "@cf/baai/bge-m3")
}

func TestResolveUpstreamModel(t *testing.T) {
	t.Parallel()

	for _, spec := range modelSpecs {
		require.Equal(t, spec.UpstreamID, resolveUpstreamModel(spec.ID))
		require.Equal(t, spec.UpstreamID, resolveUpstreamModel(spec.UpstreamID))
	}
	require.Equal(t, "custom-model", resolveUpstreamModel("custom-model"))
}
