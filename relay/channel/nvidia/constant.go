package nvidia

const (
	ChannelName = "nvidia"

	ModelLlamaNemotronEmbed1BV2           = "llama-nemotron-embed-1b-v2"
	ModelLlama32Nemoretriever300MEmbedV2  = "llama-3_2-nemoretriever-300m-embed-v2"
	ModelLlama32Nemoretriever300MEmbedV1  = "llama-3_2-nemoretriever-300m-embed-v1"
	ModelLlama32NvEmbedqa1BV2             = "llama-3.2-nv-embedqa-1b-v2"
	ModelNvEmbedqaE5V5                    = "nv-embedqa-e5-v5"
	ModelNvEmbedV1                        = "nv-embed-v1"
	ModelBgeM3                            = "bge-m3"
	ModelLlamaNemotronEmbedVl1BV2         = "llama-nemotron-embed-vl-1b-v2"
	ModelLlama32Nemoretriever1BVlmEmbedV1 = "llama-3.2-nemoretriever-1b-vlm-embed-v1"
	ModelNVDinoV2                         = "nv-dinov2"
)

var ModelList = []string{
	ModelLlamaNemotronEmbed1BV2,
	ModelLlama32Nemoretriever300MEmbedV2,
	ModelLlama32Nemoretriever300MEmbedV1,
	ModelLlama32NvEmbedqa1BV2,
	ModelNvEmbedqaE5V5,
	ModelNvEmbedV1,
	ModelBgeM3,
	ModelLlamaNemotronEmbedVl1BV2,
	ModelLlama32Nemoretriever1BVlmEmbedV1,
	ModelNVDinoV2,
}

var modelSet = map[string]struct{}{
	ModelLlamaNemotronEmbed1BV2:           {},
	ModelLlama32Nemoretriever300MEmbedV2:  {},
	ModelLlama32Nemoretriever300MEmbedV1:  {},
	ModelLlama32NvEmbedqa1BV2:             {},
	ModelNvEmbedqaE5V5:                    {},
	ModelNvEmbedV1:                        {},
	ModelBgeM3:                            {},
	ModelLlamaNemotronEmbedVl1BV2:         {},
	ModelLlama32Nemoretriever1BVlmEmbedV1: {},
	ModelNVDinoV2:                         {},
}

func IsSupportedModel(model string) bool {
	_, ok := modelSet[model]
	return ok
}

func IsNVDinoV2Model(model string) bool {
	return model == ModelNVDinoV2
}
