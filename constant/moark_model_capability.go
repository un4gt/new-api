package constant

import "strings"

var moarkEmbeddingOnlyModels = map[string]struct{}{
	"nomic-embed-code": {},
}

var moarkMultimodalEmbeddingModels = map[string]struct{}{
	"Qwen3-VL-Embedding-2B": {},
	"Qwen3-VL-Embedding-8B": {},
	"jina-clip-v1":         {},
	"jina-clip-v2":         {},
	"jina-embeddings-v4":   {},
}

var moarkRerankModels = map[string]struct{}{
	"bge-reranker-v2-m3": {},
}

var moarkMultimodalRerankModels = map[string]struct{}{
	"Qwen3-VL-Reranker-2B": {},
	"Qwen3-VL-Reranker-8B": {},
	"jina-reranker-m0":     {},
}

func normalizeMoarkModelName(model string) string {
	return strings.TrimSpace(model)
}

func IsMoarkEmbeddingModel(model string) bool {
	model = normalizeMoarkModelName(model)
	if _, ok := moarkEmbeddingOnlyModels[model]; ok {
		return true
	}
	if _, ok := moarkMultimodalEmbeddingModels[model]; ok {
		return true
	}
	return false
}

func IsMoarkMultimodalEmbeddingModel(model string) bool {
	model = normalizeMoarkModelName(model)
	_, ok := moarkMultimodalEmbeddingModels[model]
	return ok
}

func IsMoarkRerankModel(model string) bool {
	model = normalizeMoarkModelName(model)
	if _, ok := moarkRerankModels[model]; ok {
		return true
	}
	if _, ok := moarkMultimodalRerankModels[model]; ok {
		return true
	}
	return false
}

func IsMoarkMultimodalRerankModel(model string) bool {
	model = normalizeMoarkModelName(model)
	_, ok := moarkMultimodalRerankModels[model]
	return ok
}

func GetMoarkEndpointTypes(model string) []EndpointType {
	model = normalizeMoarkModelName(model)
	switch {
	case IsMoarkEmbeddingModel(model):
		return []EndpointType{EndpointTypeEmbeddings}
	case IsMoarkMultimodalRerankModel(model):
		return []EndpointType{EndpointTypeRerankMultimodal}
	case IsMoarkRerankModel(model):
		return []EndpointType{EndpointTypeJinaRerank}
	default:
		return nil
	}
}
