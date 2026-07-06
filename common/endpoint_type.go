package common

import (
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

// GetEndpointTypesByChannelType 获取渠道最优先端点类型（所有的渠道都支持 OpenAI 端点）
func GetEndpointTypesByChannelType(channelType int, modelName string) []constant.EndpointType {
	var endpointTypes []constant.EndpointType
	switch channelType {
	case constant.ChannelTypeJina:
		endpointTypes = []constant.EndpointType{constant.EndpointTypeJinaRerank}
	case constant.ChannelTypeMoark:
		if preciseEndpointTypes := constant.GetMoarkEndpointTypes(modelName); len(preciseEndpointTypes) > 0 {
			endpointTypes = preciseEndpointTypes
			break
		}
		endpointTypes = []constant.EndpointType{
			constant.EndpointTypeEmbeddings,
			constant.EndpointTypeJinaRerank,
			constant.EndpointTypeSentenceSimilarity,
			constant.EndpointTypeRerankMultimodal,
		}
	case constant.ChannelTypeNvidia:
		endpointTypes = []constant.EndpointType{
			constant.EndpointTypeEmbeddings,
		}
	case constant.ChannelTypeCohere:
		if strings.HasPrefix(modelName, "embed-") {
			endpointTypes = []constant.EndpointType{constant.EndpointTypeEmbeddings}
		} else if strings.HasPrefix(modelName, "rerank-") {
			endpointTypes = []constant.EndpointType{constant.EndpointTypeJinaRerank}
		} else {
			endpointTypes = []constant.EndpointType{
				constant.EndpointTypeEmbeddings,
				constant.EndpointTypeJinaRerank,
			}
		}
	//case constant.ChannelTypeSunoAPI:
	//	endpointTypes = []constant.EndpointType{constant.EndpointTypeSuno}
	//case constant.ChannelTypeKling:
	//	endpointTypes = []constant.EndpointType{constant.EndpointTypeKling}
	//case constant.ChannelTypeJimeng:
	//	endpointTypes = []constant.EndpointType{constant.EndpointTypeJimeng}
	case constant.ChannelTypeVertexAi:
		fallthrough
	case constant.ChannelTypeGemini:
		endpointTypes = []constant.EndpointType{constant.EndpointTypeGemini, constant.EndpointTypeOpenAI}
	case constant.ChannelTypeOpenRouter:
		if modelName == constant.OpenRouterGeminiEmbedding2PreviewModel {
			endpointTypes = []constant.EndpointType{constant.EndpointTypeEmbeddings}
		} else {
			endpointTypes = []constant.EndpointType{constant.EndpointTypeOpenAI}
		}
	case constant.ChannelTypeXai:
		endpointTypes = []constant.EndpointType{constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse}
	case constant.ChannelTypeSora:
		endpointTypes = []constant.EndpointType{constant.EndpointTypeOpenAIVideo}
	default:
		if IsOpenAIResponseOnlyModel(modelName) {
			endpointTypes = []constant.EndpointType{constant.EndpointTypeOpenAIResponse}
		} else {
			endpointTypes = []constant.EndpointType{constant.EndpointTypeOpenAI}
		}
	}
	if IsImageGenerationModel(modelName) {
		// add to first
		endpointTypes = append([]constant.EndpointType{constant.EndpointTypeImageGeneration}, endpointTypes...)
	}
	return endpointTypes
}
