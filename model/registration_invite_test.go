package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateRegistrationInviteCodeUsesPrefix(t *testing.T) {
	code, hash, err := generateRegistrationInviteCode()

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(code, registrationInviteCodePrefix))
	assert.Len(t, code, 32)
	assert.Equal(t, hashRegistrationInviteCode(code), hash)
}
