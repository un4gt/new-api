package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestCloudflareCombinedCredentialIsOneMultiKeyUnit(t *testing.T) {
	t.Parallel()

	channel := &Channel{
		Key: "account-1,token-1\naccount-2,token-2",
		ChannelInfo: ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 2,
			MultiKeyMode: constant.MultiKeyModeRandom,
		},
	}

	handlerMultiKeyUpdate(channel, "account-1,token-1", 2, "test disable")
	require.Equal(t, 2, channel.ChannelInfo.MultiKeyStatusList[0])
	_, secondDisabled := channel.ChannelInfo.MultiKeyStatusList[1]
	require.False(t, secondDisabled)

	key, index, apiErr := channel.GetNextEnabledKey()
	require.Nil(t, apiErr)
	require.Equal(t, 1, index)
	require.Equal(t, "account-2,token-2", key)
}
