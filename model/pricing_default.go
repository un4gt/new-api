package model

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

// 简化的供应商映射规则
var defaultVendorRules = map[string]string{
	"gpt":      "OpenAI",
	"dall-e":   "OpenAI",
	"whisper":  "OpenAI",
	"o1":       "OpenAI",
	"o3":       "OpenAI",
	"claude":   "Anthropic",
	"gemini":   "Google",
	"moonshot": "Moonshot",
	"kimi":     "Moonshot",
	"chatglm":  "智谱",
	"glm-":     "智谱",
	"qwen":     "阿里巴巴",
	"deepseek": "DeepSeek",
	"abab":     "MiniMax",
	"ernie":    "百度",
	"spark":    "讯飞",
	"hunyuan":  "腾讯",
	"command":  "Cohere",
	"@cf/":     "Cloudflare",
	"360":      "360",
	"yi":       "零一万物",
	"jina":     "Jina",
	"mistral":  "Mistral",
	"grok":     "xAI",
	"llama":    "Meta",
	"doubao":   "字节跳动",
	"kling":    "快手",
	"jimeng":   "即梦",
	"vidu":     "Vidu",
	"nvidia":   "NVIDIA",
	"nv-":      "NVIDIA",
	"nemotron": "NVIDIA",
	"dinov2":   "NVIDIA",
	"zembed":   "ZeroEntropy",
	"zerank":   "ZeroEntropy",
}

// 供应商默认图标映射
var defaultVendorIcons = map[string]string{
	"OpenAI":     "OpenAI",
	"Anthropic":  "Claude.Color",
	"Google":     "Gemini.Color",
	"Moonshot":   "Moonshot",
	"智谱":         "Zhipu.Color",
	"阿里巴巴":       "Qwen.Color",
	"DeepSeek":   "DeepSeek.Color",
	"MiniMax":    "Minimax.Color",
	"百度":         "Wenxin.Color",
	"讯飞":         "Spark.Color",
	"腾讯":         "Hunyuan.Color",
	"Cohere":     "Cohere.Color",
	"Cloudflare": "Cloudflare.Color",
	"360":        "Ai360.Color",
	"零一万物":       "Yi.Color",
	"Jina":       "Jina",
	"Mistral":    "Mistral.Color",
	"xAI":        "XAI",
	"Meta":       "Ollama",
	"字节跳动":       "Doubao.Color",
	"快手":         "Kling.Color",
	"即梦":         "Jimeng.Color",
	"Vidu":       "Vidu",
	"微软":         "AzureAI",
	"Microsoft":  "AzureAI",
	"Azure":      "AzureAI",
	"Elastic":    "Elastic",
	"NVIDIA":     "SiNvidia",
	"ZeroEntropy": "ZeroEntropy",
}

// Explicit metadata defaults for NVIDIA models to keep UI display consistent.
// These are applied only when the model has no existing metadata row.
var nvidiaModelMetadataDefaults = map[string]struct {
	Description string
	Tags        string
	Icon        string
}{
	"llama-nemotron-embed-1b-v2": {
		Description: "NVIDIA text embedding model for retrieval and semantic similarity.",
		Tags:        "NVIDIA,text-to-embedding,embedding,retrieval",
		Icon:        "SiNvidia",
	},
	"llama-3_2-nemoretriever-300m-embed-v2": {
		Description: "NVIDIA text embedding model optimized for retrieval workloads.",
		Tags:        "NVIDIA,text-to-embedding,embedding,retrieval",
		Icon:        "SiNvidia",
	},
	"llama-3.2-nemoretriever-300m-embed-v2": {
		Description: "NVIDIA text embedding model optimized for retrieval workloads.",
		Tags:        "NVIDIA,text-to-embedding,embedding,retrieval",
		Icon:        "SiNvidia",
	},
	"llama-3_2-nemoretriever-300m-embed-v1": {
		Description: "NVIDIA text embedding model for retrieval and ranking scenarios.",
		Tags:        "NVIDIA,text-to-embedding,embedding,retrieval",
		Icon:        "SiNvidia",
	},
	"llama-3.2-nemoretriever-300m-embed-v1": {
		Description: "NVIDIA text embedding model for retrieval and ranking scenarios.",
		Tags:        "NVIDIA,text-to-embedding,embedding,retrieval",
		Icon:        "SiNvidia",
	},
	"llama-3.2-nv-embedqa-1b-v2": {
		Description: "NVIDIA text embedding model tuned for embedding-based question answering.",
		Tags:        "NVIDIA,text-to-embedding,embedding,qa",
		Icon:        "SiNvidia",
	},
	"nv-embedqa-e5-v5": {
		Description: "NVIDIA text embedding model for retrieval and QA pipelines.",
		Tags:        "NVIDIA,text-to-embedding,embedding,qa",
		Icon:        "SiNvidia",
	},
	"nv-embed-v1": {
		Description: "NVIDIA general-purpose text embedding model.",
		Tags:        "NVIDIA,text-to-embedding,embedding",
		Icon:        "SiNvidia",
	},
	"bge-m3": {
		Description: "BGE-M3 embedding model exposed via NVIDIA Build and NVIDIA channel.",
		Tags:        "NVIDIA,text-to-embedding,embedding,multilingual",
		Icon:        "SiNvidia",
	},
	"baai/bge-m3": {
		Description: "BGE-M3 embedding model exposed via NVIDIA Build and NVIDIA channel.",
		Tags:        "NVIDIA,text-to-embedding,embedding,multilingual",
		Icon:        "SiNvidia",
	},
	"llama-nemotron-embed-vl-1b-v2": {
		Description: "NVIDIA multimodal embedding model for image-text retrieval.",
		Tags:        "NVIDIA,multimodal-embedding,image-to-embedding,embedding",
		Icon:        "SiNvidia",
	},
	"llama-3.2-nemoretriever-1b-vlm-embed-v1": {
		Description: "NVIDIA multimodal embedding model for visual-language retrieval.",
		Tags:        "NVIDIA,multimodal-embedding,image-to-embedding,embedding",
		Icon:        "SiNvidia",
	},
	"nv-dinov2": {
		Description: "NVIDIA image embedding model with base64 (<200KB) and NVCF asset_id (>=200KB) input modes.",
		Tags:        "NVIDIA,image-to-embedding,multimodal-embedding,embedding",
		Icon:        "SiNvidia",
	},
	"nvidia/llama-nemotron-embed-1b-v2": {
		Description: "NVIDIA text embedding model for retrieval and semantic similarity.",
		Tags:        "NVIDIA,text-to-embedding,embedding,retrieval",
		Icon:        "SiNvidia",
	},
	"nvidia/llama-3_2-nemoretriever-300m-embed-v2": {
		Description: "NVIDIA text embedding model optimized for retrieval workloads.",
		Tags:        "NVIDIA,text-to-embedding,embedding,retrieval",
		Icon:        "SiNvidia",
	},
	"nvidia/llama-3.2-nemoretriever-300m-embed-v2": {
		Description: "NVIDIA text embedding model optimized for retrieval workloads.",
		Tags:        "NVIDIA,text-to-embedding,embedding,retrieval",
		Icon:        "SiNvidia",
	},
	"nvidia/llama-3_2-nemoretriever-300m-embed-v1": {
		Description: "NVIDIA text embedding model for retrieval and ranking scenarios.",
		Tags:        "NVIDIA,text-to-embedding,embedding,retrieval",
		Icon:        "SiNvidia",
	},
	"nvidia/llama-3.2-nemoretriever-300m-embed-v1": {
		Description: "NVIDIA text embedding model for retrieval and ranking scenarios.",
		Tags:        "NVIDIA,text-to-embedding,embedding,retrieval",
		Icon:        "SiNvidia",
	},
	"nvidia/llama-3.2-nv-embedqa-1b-v2": {
		Description: "NVIDIA text embedding model tuned for embedding-based question answering.",
		Tags:        "NVIDIA,text-to-embedding,embedding,qa",
		Icon:        "SiNvidia",
	},
	"nvidia/llama-3.2-nv-embedqa-1b-v1": {
		Description: "NVIDIA text embedding model tuned for embedding-based question answering.",
		Tags:        "NVIDIA,text-to-embedding,embedding,qa",
		Icon:        "SiNvidia",
	},
	"nvidia/nv-embedqa-e5-v5": {
		Description: "NVIDIA text embedding model for retrieval and QA pipelines.",
		Tags:        "NVIDIA,text-to-embedding,embedding,qa",
		Icon:        "SiNvidia",
	},
	"nvidia/nv-embed-v1": {
		Description: "NVIDIA general-purpose text embedding model.",
		Tags:        "NVIDIA,text-to-embedding,embedding",
		Icon:        "SiNvidia",
	},
	"nvidia/nv-embedcode-7b-v1": {
		Description: "NVIDIA embedding model optimized for code and text retrieval.",
		Tags:        "NVIDIA,text-to-embedding,embedding,code-retrieval",
		Icon:        "SiNvidia",
	},
	"nvidia/embed-qa-4": {
		Description: "NVIDIA embedding model optimized for question-answering retrieval.",
		Tags:        "NVIDIA,text-to-embedding,embedding,qa",
		Icon:        "SiNvidia",
	},
	"nvidia/llama-nemotron-embed-vl-1b-v2": {
		Description: "NVIDIA multimodal embedding model for image-text retrieval.",
		Tags:        "NVIDIA,multimodal-embedding,image-to-embedding,embedding",
		Icon:        "SiNvidia",
	},
	"nvidia/llama-3.2-nemoretriever-1b-vlm-embed-v1": {
		Description: "NVIDIA multimodal embedding model for visual-language retrieval.",
		Tags:        "NVIDIA,multimodal-embedding,image-to-embedding,embedding",
		Icon:        "SiNvidia",
	},
	"nvidia/nv-dinov2": {
		Description: "NVIDIA image embedding model with base64 (<200KB) and NVCF asset_id (>=200KB) input modes.",
		Tags:        "NVIDIA,image-to-embedding,multimodal-embedding,embedding",
		Icon:        "SiNvidia",
	},
}

// initDefaultVendorMapping 简化的默认供应商映射
func initDefaultVendorMapping(metaMap map[string]*Model, vendorMap map[int]*Vendor, enableAbilities []AbilityWithChannel) {
	for _, ability := range enableAbilities {
		modelName := ability.Model
		if _, exists := metaMap[modelName]; exists {
			continue
		}

		// 匹配供应商
		vendorID := 0
		modelLower := strings.ToLower(modelName)
		for pattern, vendorName := range defaultVendorRules {
			if strings.Contains(modelLower, pattern) {
				vendorID = getOrCreateVendor(vendorName, vendorMap)
				break
			}
		}

		// 创建模型元数据
		description := ""
		tags := ""
		icon := ""
		if ability.ChannelType == constant.ChannelTypeNvidia {
			if metaDefaults, ok := nvidiaModelMetadataDefaults[modelName]; ok {
				description = metaDefaults.Description
				tags = metaDefaults.Tags
				icon = metaDefaults.Icon
			}
		}

		metaMap[modelName] = &Model{
			ModelName:   modelName,
			Description: description,
			Tags:        tags,
			Icon:        icon,
			VendorID:    vendorID,
			Status:      1,
			NameRule:    NameRuleExact,
		}
	}
}

// 查找或创建供应商
func getOrCreateVendor(vendorName string, vendorMap map[int]*Vendor) int {
	// 查找现有供应商
	for id, vendor := range vendorMap {
		if vendor.Name == vendorName {
			return id
		}
	}

	// 创建新供应商
	newVendor := &Vendor{
		Name:   vendorName,
		Status: 1,
		Icon:   getDefaultVendorIcon(vendorName),
	}

	if err := newVendor.Insert(); err != nil {
		return 0
	}

	vendorMap[newVendor.Id] = newVendor
	return newVendor.Id
}

// 获取供应商默认图标
func getDefaultVendorIcon(vendorName string) string {
	if icon, exists := defaultVendorIcons[vendorName]; exists {
		return icon
	}
	return ""
}
