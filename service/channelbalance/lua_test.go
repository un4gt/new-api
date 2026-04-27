package channelbalance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQueryWithScriptInjectsCredentialsAndExtractsBalance(t *testing.T) {
	t.Parallel()

	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		require.Equal(t, "/balance", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":{"normal_balance":12300000}}`))
	}))
	defer server.Close()

	script := `
request = {
  url = base_url .. "/balance",
  method = "GET",
  headers = {
    Authorization = "Bearer " .. apikey
  }
}

function extractor(response)
  return {
    remaining = response.user.normal_balance / 1000000,
    unit = "USD"
  }
end
`

	result, err := QueryWithScript(context.Background(), script, Params{
		BaseURL:         server.URL,
		APIKey:          "test-key",
		HTTPClient:      server.Client(),
		AllowPrivateURL: true,
	})

	require.NoError(t, err)
	require.Equal(t, "Bearer test-key", gotAuth)
	require.InDelta(t, 12.3, result.Remaining, 1e-12)
	require.Equal(t, "USD", result.Unit)
}

func TestQueryWithScriptRequiresExtractor(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"balance":10}`))
	}))
	defer server.Close()

	script := `
request = {
  url = base_url,
  method = "GET"
}
`

	_, err := QueryWithScript(context.Background(), script, Params{
		BaseURL:         server.URL,
		HTTPClient:      server.Client(),
		AllowPrivateURL: true,
	})
	require.ErrorContains(t, err, "extractor")
}

func TestQueryWithScriptRequiresNumericRemaining(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"balance":10}`))
	}))
	defer server.Close()

	script := `
request = {
  url = base_url,
  method = "GET"
}

function extractor(response)
  return {
    remaining = "10",
    unit = "USD"
  }
end
`

	_, err := QueryWithScript(context.Background(), script, Params{
		BaseURL:         server.URL,
		HTTPClient:      server.Client(),
		AllowPrivateURL: true,
	})
	require.ErrorContains(t, err, "remaining")
}

func TestQueryWithScriptTimesOutLuaExecution(t *testing.T) {
	t.Parallel()

	script := `
while true do
end
`

	_, err := QueryWithScript(context.Background(), script, Params{
		Timeout: 20 * time.Millisecond,
	})
	require.Error(t, err)
}
