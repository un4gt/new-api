package openrouter

import "github.com/QuantumNous/new-api/constant"

const GeminiEmbedding2PreviewModel = constant.OpenRouterGeminiEmbedding2PreviewModel

var ModelList = []string{
	GeminiEmbedding2PreviewModel,
	"cohere/rerank-4-pro",
	"cohere/rerank-4-fast",
	"cohere/rerank-v3.5",
}

var ChannelName = "openrouter"
