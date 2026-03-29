package nvidia

import "strings"

const (
	ChannelName = "nvidia"

	ModelLlamaNemotronEmbed1BV2           = "nvidia/llama-nemotron-embed-1b-v2"
	ModelLlama32Nemoretriever300MEmbedV2  = "nvidia/llama-3.2-nemoretriever-300m-embed-v2"
	ModelLlama32Nemoretriever300MEmbedV1  = "nvidia/llama-3.2-nemoretriever-300m-embed-v1"
	ModelLlama32NvEmbedqa1BV2             = "nvidia/llama-3.2-nv-embedqa-1b-v2"
	ModelLlama32NvEmbedqa1BV1             = "nvidia/llama-3.2-nv-embedqa-1b-v1"
	ModelNvEmbedqaE5V5                    = "nvidia/nv-embedqa-e5-v5"
	ModelNvEmbedV1                        = "nvidia/nv-embed-v1"
	ModelBgeM3                            = "baai/bge-m3"
	ModelNvEmbedCode7BV1                  = "nvidia/nv-embedcode-7b-v1"
	ModelEmbedQA4                         = "nvidia/embed-qa-4"
	ModelLlamaNemotronEmbedVl1BV2         = "nvidia/llama-nemotron-embed-vl-1b-v2"
	ModelLlama32Nemoretriever1BVlmEmbedV1 = "nvidia/llama-3.2-nemoretriever-1b-vlm-embed-v1"
	ModelNVDinoV2                         = "nvidia/nv-dinov2"
)

var ModelList = []string{
	ModelLlamaNemotronEmbed1BV2,
	ModelLlama32Nemoretriever300MEmbedV2,
	ModelLlama32Nemoretriever300MEmbedV1,
	ModelLlama32NvEmbedqa1BV2,
	ModelLlama32NvEmbedqa1BV1,
	ModelNvEmbedqaE5V5,
	ModelNvEmbedV1,
	ModelBgeM3,
	ModelNvEmbedCode7BV1,
	ModelEmbedQA4,
	ModelLlamaNemotronEmbedVl1BV2,
	ModelLlama32Nemoretriever1BVlmEmbedV1,
	ModelNVDinoV2,
}

var modelAliasToCanonical = map[string]string{
	ModelLlamaNemotronEmbed1BV2:                      ModelLlamaNemotronEmbed1BV2,
	"llama-nemotron-embed-1b-v2":                     ModelLlamaNemotronEmbed1BV2,
	ModelLlama32Nemoretriever300MEmbedV2:             ModelLlama32Nemoretriever300MEmbedV2,
	"nvidia/llama-3_2-nemoretriever-300m-embed-v2":   ModelLlama32Nemoretriever300MEmbedV2,
	"llama-3_2-nemoretriever-300m-embed-v2":          ModelLlama32Nemoretriever300MEmbedV2,
	"llama-3.2-nemoretriever-300m-embed-v2":          ModelLlama32Nemoretriever300MEmbedV2,
	ModelLlama32Nemoretriever300MEmbedV1:             ModelLlama32Nemoretriever300MEmbedV1,
	"nvidia/llama-3_2-nemoretriever-300m-embed-v1":   ModelLlama32Nemoretriever300MEmbedV1,
	"llama-3_2-nemoretriever-300m-embed-v1":          ModelLlama32Nemoretriever300MEmbedV1,
	"llama-3.2-nemoretriever-300m-embed-v1":          ModelLlama32Nemoretriever300MEmbedV1,
	ModelLlama32NvEmbedqa1BV2:                        ModelLlama32NvEmbedqa1BV2,
	"llama-3.2-nv-embedqa-1b-v2":                     ModelLlama32NvEmbedqa1BV2,
	ModelLlama32NvEmbedqa1BV1:                        ModelLlama32NvEmbedqa1BV1,
	"llama-3.2-nv-embedqa-1b-v1":                     ModelLlama32NvEmbedqa1BV1,
	ModelNvEmbedqaE5V5:                               ModelNvEmbedqaE5V5,
	"nv-embedqa-e5-v5":                               ModelNvEmbedqaE5V5,
	ModelNvEmbedV1:                                   ModelNvEmbedV1,
	"nv-embed-v1":                                    ModelNvEmbedV1,
	ModelBgeM3:                                       ModelBgeM3,
	"bge-m3":                                         ModelBgeM3,
	ModelNvEmbedCode7BV1:                             ModelNvEmbedCode7BV1,
	"nv-embedcode-7b-v1":                             ModelNvEmbedCode7BV1,
	ModelEmbedQA4:                                    ModelEmbedQA4,
	"embed-qa-4":                                     ModelEmbedQA4,
	ModelLlamaNemotronEmbedVl1BV2:                    ModelLlamaNemotronEmbedVl1BV2,
	"llama-nemotron-embed-vl-1b-v2":                  ModelLlamaNemotronEmbedVl1BV2,
	ModelLlama32Nemoretriever1BVlmEmbedV1:            ModelLlama32Nemoretriever1BVlmEmbedV1,
	"nvidia/llama-3_2-nemoretriever-1b-vlm-embed-v1": ModelLlama32Nemoretriever1BVlmEmbedV1,
	"llama-3.2-nemoretriever-1b-vlm-embed-v1":        ModelLlama32Nemoretriever1BVlmEmbedV1,
	"llama-3_2-nemoretriever-1b-vlm-embed-v1":        ModelLlama32Nemoretriever1BVlmEmbedV1,
	ModelNVDinoV2:                                    ModelNVDinoV2,
	"nv-dinov2":                                      ModelNVDinoV2,
}

func NormalizeModel(model string) (string, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	canonical, ok := modelAliasToCanonical[model]
	return canonical, ok
}

func IsSupportedModel(model string) bool {
	_, ok := NormalizeModel(model)
	return ok
}

func IsNVDinoV2Model(model string) bool {
	canonical, ok := NormalizeModel(model)
	return ok && canonical == ModelNVDinoV2
}
