package elastic

var ModelList = []string{
	// Embedding models
	"jina-clip-v2",
	"jina-embeddings-v3",
	"jina-embeddings-v5-text-nano",
	"jina-embeddings-v5-text-small",
	"google-gemini-embedding-001",
	"openai-text-embedding-3-large",
	"openai-text-embedding-3-small",

	// Rerank models
	"jina-reranker-v2-base-multilingual",
	"jina-reranker-v3",
}

var ChannelName = "elastic_inference_endpoints"

var hostedInferenceModelNameSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(ModelList))
	for _, name := range ModelList {
		if name == "" {
			continue
		}
		set[name] = struct{}{}
	}
	return set
}()

func isHostedInferenceModelName(name string) bool {
	_, ok := hostedInferenceModelNameSet[name]
	return ok
}
