//go:build legacy_db
// +build legacy_db

package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSearchRedemptionsMatchesRedemptionKey(t *testing.T) {
	truncateTables(t)

	redemption := &Redemption{
		UserId:      1,
		Key:         "abc123-redemption-key",
		Status:      common.RedemptionCodeStatusEnabled,
		Name:        "search-by-name",
		Quota:       100,
		CreatedTime: 10,
	}
	require.NoError(t, DB.Create(redemption).Error)

	redemptions, total, err := SearchRedemptions("redemption-key", 0, 10)

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, redemptions, 1)
	assert.Equal(t, redemption.Id, redemptions[0].Id)
	assert.Equal(t, "abc123-redemption-key", redemptions[0].Key)
}

func TestCreateAndSearchRegistrationInvitesByCode(t *testing.T) {
	truncateTables(t)

	invites, err := CreateRegistrationInvites(7, "spring launch", 0, 1, 3)
	require.NoError(t, err)
	require.Len(t, invites, 3)

	for _, item := range invites {
		require.NotNil(t, item.Invite)
		assert.NotEmpty(t, item.Code)
		assert.Equal(t, item.Code, item.Invite.Code)
		assert.Equal(t, common.RegistrationInviteStatusActive, item.Invite.Status)
		assert.Equal(t, 1, item.Invite.MaxUses)
	}

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	found, total, err := SearchRegistrationInvites(invites[1].Code, pageInfo)

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, found, 1)
	assert.Equal(t, invites[1].Invite.Id, found[0].Id)
	assert.Equal(t, invites[1].Code, found[0].Code)
}

func TestConsumeRegistrationInviteStoresUsageDetails(t *testing.T) {
	truncateTables(t)

	invite, code, err := CreateRegistrationInvite(1, "usage visible", 0, 1)
	require.NoError(t, err)
	require.NotEmpty(t, code)

	err = DB.Transaction(func(tx *gorm.DB) error {
		return ConsumeRegistrationInviteTx(tx, invite.Id, 42, "password", "new-user")
	})
	require.NoError(t, err)

	pageInfo := &common.PageInfo{Page: 1, PageSize: 10}
	found, total, err := SearchRegistrationInvites(code, pageInfo)

	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, found, 1)
	assert.Equal(t, common.RegistrationInviteStatusUsed, found[0].Status)
	assert.Equal(t, 1, found[0].UseCount)
	assert.Equal(t, 42, found[0].UsedBy)
	assert.NotZero(t, found[0].UsedAt)
	assert.Equal(t, "password", found[0].UsedProvider)
	assert.Equal(t, "new-user", found[0].UsedProviderUserID)
}

func TestCreateRegistrationInvitesRejectsInvalidCount(t *testing.T) {
	truncateTables(t)

	_, err := CreateRegistrationInvites(1, "", 0, 1, 101)

	assert.True(t, errors.Is(err, ErrRegistrationInviteCountInvalid))
}
