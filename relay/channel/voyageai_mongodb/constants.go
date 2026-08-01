package voyageai_mongodb

const (
	ModelVoyageMultimodal35 = "voyage-multimodal-3.5"
	ChannelName             = "voyageai-by-mongodb"
)

var ModelList = []string{
	// Text embedding models.
	"voyage-4-large",
	"voyage-4",
	"voyage-4-lite",
	"voyage-code-3",
	"voyage-4-nano",

	// Multimodal embedding models.
	ModelVoyageMultimodal35,

	// Rerank models.
	"rerank-2.5",
	"rerank-2.5-lite",
}
