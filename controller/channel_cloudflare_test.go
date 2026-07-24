package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestValidateChannelCloudflareCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		key       string
		isAdd     bool
		wantError bool
	}{
		{name: "single", key: "account-1,token-1", isAdd: true},
		{name: "multi key aggregation", key: "account-1,token-1\naccount-2,token-2", isAdd: true},
		{name: "trimmed parts", key: " account-1 , token-1 ", isAdd: true},
		{name: "legacy token", key: "legacy-token", isAdd: true, wantError: true},
		{name: "missing account id", key: ",token", isAdd: true, wantError: true},
		{name: "missing token", key: "account,", isAdd: true, wantError: true},
		{name: "invalid second key", key: "account-1,token-1\nlegacy-token", isAdd: true, wantError: true},
		{name: "empty key allowed on update", key: "", isAdd: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			channel := &model.Channel{Type: constant.ChannelCloudflare, Key: test.key}
			err := validateChannel(channel, test.isAdd)
			if test.wantError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "Cloudflare")
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestValidateCloudflareKeyForMode(t *testing.T) {
	t.Parallel()

	channel := &model.Channel{
		Type: constant.ChannelCloudflare,
		Key:  "account-1,token-1\naccount-2,token-2",
	}
	require.Error(t, validateCloudflareKeyForMode(channel, false))
	require.NoError(t, validateCloudflareKeyForMode(channel, true))
	require.Equal(t, "account-1,token-1\naccount-2,token-2", channel.Key)

	channel.Key = " account-1 , token-1 "
	require.NoError(t, validateCloudflareKeyForMode(channel, false))
	require.Equal(t, "account-1,token-1", channel.Key)
}
