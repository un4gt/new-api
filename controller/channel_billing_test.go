package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMoarkBalanceURL(t *testing.T) {
	t.Parallel()

	require.Equal(t,
		"https://ai.gitee.com/v1/tokens/packages/balance",
		moarkBalanceURL("https://ai.gitee.com"),
	)
	require.Equal(t,
		"https://ai.gitee.com/v1/tokens/packages/balance",
		moarkBalanceURL("https://ai.gitee.com/"),
	)
	require.Equal(t,
		"https://ai.gitee.com/v1/tokens/packages/balance",
		moarkBalanceURL("https://ai.gitee.com/v1"),
	)
}

func TestParseMoarkBalance(t *testing.T) {
	t.Parallel()

	body := []byte(`{
		"total_amount": 12,
		"used_amount": 3.25094514,
		"balance": 8.74905486,
		"details": [
			{
				"ident": "QUDWYD3QUO13",
				"name": "全模型资源包",
				"amount": 12,
				"balance": 8.74905486
			}
		]
	}`)

	balance, err := parseMoarkBalance(body)
	require.NoError(t, err)
	require.InDelta(t, 8.74905486, balance, 1e-12)
}
