package cloudflare

type modelSpec struct {
	ID            string
	UpstreamID    string
	SupportsBatch bool
}

var modelSpecs = []modelSpec{
	{ID: "cf/plamo-embedding-1b", UpstreamID: "@cf/pfnet/plamo-embedding-1b", SupportsBatch: false},
	{ID: "cf/embeddinggemma-300m", UpstreamID: "@cf/google/embeddinggemma-300m", SupportsBatch: false},
	{ID: "cf/qwen-embedding-0.6b", UpstreamID: "@cf/qwen/qwen3-embedding-0.6b", SupportsBatch: false},
	{ID: "cf/bge-m3", UpstreamID: "@cf/baai/bge-m3", SupportsBatch: true},
	{ID: "cf/bge-large-en-v1.5", UpstreamID: "@cf/baai/bge-large-en-v1.5", SupportsBatch: true},
	{ID: "cf/bge-small-en-v1.5", UpstreamID: "@cf/baai/bge-small-en-v1.5", SupportsBatch: true},
	{ID: "cf/bge-base-en-v1.5", UpstreamID: "@cf/baai/bge-base-en-v1.5", SupportsBatch: true},
}

func resolveUpstreamModel(model string) string {
	for _, spec := range modelSpecs {
		if model == spec.ID {
			return spec.UpstreamID
		}
	}
	return model
}

var ModelList = func() []string {
	models := make([]string, 0, len(modelSpecs))
	for _, spec := range modelSpecs {
		models = append(models, spec.ID)
	}
	return models
}()

var ChannelName = "cloudflare"
