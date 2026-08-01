package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestGetAdaptorVoyageAIByMongoDB(t *testing.T) {
	t.Parallel()

	adaptor := GetAdaptor(constant.APITypeVoyageAIByMongoDB)
	if adaptor == nil {
		t.Fatal("VoyageAIByMongoDB adaptor is not registered")
	}
	if channelName := adaptor.GetChannelName(); channelName != "voyageai-by-mongodb" {
		t.Fatalf("unexpected channel name: %s", channelName)
	}
}
