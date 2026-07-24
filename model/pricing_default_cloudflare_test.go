package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestInitDefaultVendorMappingPrefersCloudflareChannelType(t *testing.T) {
	t.Parallel()

	metaMap := make(map[string]*Model)
	vendorMap := map[int]*Vendor{
		1: {Id: 1, Name: "Cloudflare"},
		2: {Id: 2, Name: "阿里巴巴"},
	}
	abilities := []AbilityWithChannel{
		{
			Ability: Ability{
				Model: "cf/qwen-embedding-0.6b",
			},
			ChannelType: constant.ChannelCloudflare,
		},
	}

	initDefaultVendorMapping(metaMap, vendorMap, abilities)
	require.Contains(t, metaMap, "cf/qwen-embedding-0.6b")
	require.Equal(t, 1, metaMap["cf/qwen-embedding-0.6b"].VendorID)
}
