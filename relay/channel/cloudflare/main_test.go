package cloudflare

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/service"
)

func TestMain(m *testing.M) {
	service.InitHttpClient()
	os.Exit(m.Run())
}
