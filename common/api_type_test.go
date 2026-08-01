package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestChannelType2APITypeVoyageAIByMongoDB(t *testing.T) {
	t.Parallel()

	apiType, ok := ChannelType2APIType(constant.ChannelTypeVoyageAIByMongoDB)
	if !ok {
		t.Fatal("VoyageAIByMongoDB channel type is not registered")
	}
	if apiType != constant.APITypeVoyageAIByMongoDB {
		t.Fatalf("unexpected API type: got %d, want %d", apiType, constant.APITypeVoyageAIByMongoDB)
	}
}
