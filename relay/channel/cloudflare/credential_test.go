package cloudflare

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCredential(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     string
		accountID string
		apiToken  string
		wantError bool
	}{
		{name: "valid", value: "account-id,api-token", accountID: "account-id", apiToken: "api-token"},
		{name: "trims surrounding whitespace", value: "  account-id  ,  api-token  ", accountID: "account-id", apiToken: "api-token"},
		{name: "missing comma", value: "api-token", wantError: true},
		{name: "multiple commas", value: "account-id,api,token", wantError: true},
		{name: "empty account id", value: " ,api-token", wantError: true},
		{name: "empty token", value: "account-id, ", wantError: true},
		{name: "newline", value: "account-id,api-token\nother,token", wantError: true},
		{name: "carriage return", value: "account-id,api-token\r", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			accountID, apiToken, err := ParseCredential(test.value)
			if test.wantError {
				require.Error(t, err)
				require.Empty(t, accountID)
				require.Empty(t, apiToken)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.accountID, accountID)
			require.Equal(t, test.apiToken, apiToken)
		})
	}
}

func TestValidateCredentialList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     string
		wantError bool
	}{
		{name: "single", value: "account-1,token-1"},
		{name: "multiple", value: "account-1,token-1\naccount-2,token-2"},
		{name: "blank lines and CRLF", value: "\n account-1,token-1 \r\n\naccount-2,token-2\n"},
		{name: "empty", value: "\n \n", wantError: true},
		{name: "invalid second line", value: "account-1,token-1\nold-token", wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateCredentialList(test.value)
			if test.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNormalizeCredentialList(t *testing.T) {
	t.Parallel()

	credentials, err := NormalizeCredentialList("\n account-1 , token-1 \r\naccount-2,token-2\n")
	require.NoError(t, err)
	require.Equal(t, []string{"account-1,token-1", "account-2,token-2"}, credentials)
}
