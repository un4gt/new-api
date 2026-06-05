//go:build legacy_db
// +build legacy_db

package model

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createBusinessErrorTestUser(t *testing.T, quota int) *User {
	t.Helper()
	user := &User{
		Username:    "business-user",
		Password:    "password",
		DisplayName: "business-user",
		Status:      common.UserStatusEnabled,
		Quota:       quota,
	}
	require.NoError(t, DB.Create(user).Error)
	return user
}

func TestRedeemTrimsKeyAndPreservesBusinessErrors(t *testing.T) {
	truncateTables(t)
	user := createBusinessErrorTestUser(t, 10)
	redemption := &Redemption{
		Key:         "trimmed-redemption-key",
		Status:      common.RedemptionCodeStatusEnabled,
		Name:        "trim-test",
		Quota:       123,
		CreatedTime: 10,
	}
	require.NoError(t, DB.Create(redemption).Error)

	quota, err := Redeem(" \ntrimmed-redemption-key\t", user.Id)

	require.NoError(t, err)
	assert.Equal(t, 123, quota)

	var updatedUser User
	require.NoError(t, DB.First(&updatedUser, user.Id).Error)
	assert.Equal(t, 133, updatedUser.Quota)

	var updatedRedemption Redemption
	require.NoError(t, DB.First(&updatedRedemption, redemption.Id).Error)
	assert.Equal(t, common.RedemptionCodeStatusUsed, updatedRedemption.Status)
	assert.Equal(t, user.Id, updatedRedemption.UsedUserId)

	_, err = Redeem("   ", user.Id)
	assert.True(t, errors.Is(err, ErrRedemptionNotProvided))

	_, err = Redeem("missing-redemption-key", user.Id)
	assert.True(t, errors.Is(err, ErrRedemptionInvalid))

	_, err = Redeem(redemption.Key, user.Id)
	assert.True(t, errors.Is(err, ErrRedemptionUsed))
}

func TestRedeemReturnsExpiredSentinel(t *testing.T) {
	truncateTables(t)
	user := createBusinessErrorTestUser(t, 0)
	redemption := &Redemption{
		Key:         "expired-redemption-key",
		Status:      common.RedemptionCodeStatusEnabled,
		Name:        "expired-test",
		Quota:       100,
		CreatedTime: 10,
		ExpiredTime: common.GetTimestamp() - 1,
	}
	require.NoError(t, DB.Create(redemption).Error)

	_, err := Redeem(redemption.Key, user.Id)

	assert.True(t, errors.Is(err, ErrRedemptionExpired))
}

func TestRechargeAndManualCompleteTopUpReturnBusinessSentinels(t *testing.T) {
	truncateTables(t)
	user := createBusinessErrorTestUser(t, 0)
	previousQuotaPerUnit := common.QuotaPerUnit
	t.Cleanup(func() {
		common.QuotaPerUnit = previousQuotaPerUnit
	})
	common.QuotaPerUnit = 10

	assert.True(t, errors.Is(Recharge("  ", "customer"), ErrTopUpNotProvided))
	assert.True(t, errors.Is(Recharge("missing-order", "customer"), ErrTopUpOrderNotFound))

	statusOrder := &TopUp{
		UserId:  user.Id,
		TradeNo: "paid-order",
		Status:  common.TopUpStatusSuccess,
		Amount:  1,
		Money:   1,
	}
	require.NoError(t, DB.Create(statusOrder).Error)
	assert.True(t, errors.Is(Recharge(statusOrder.TradeNo, "customer"), ErrTopUpOrderStatus))

	invalidQuotaOrder := &TopUp{
		UserId:  user.Id,
		TradeNo: "invalid-quota-order",
		Status:  common.TopUpStatusPending,
		Amount:  1,
		Money:   0,
	}
	require.NoError(t, DB.Create(invalidQuotaOrder).Error)
	assert.True(t, errors.Is(Recharge(invalidQuotaOrder.TradeNo, "customer"), ErrTopUpInvalidQuota))

	assert.True(t, errors.Is(ManualCompleteTopUp("missing-manual-order"), ErrTopUpOrderNotFound))
	manualStatusOrder := &TopUp{
		UserId:  user.Id,
		TradeNo: "expired-manual-order",
		Status:  common.TopUpStatusExpired,
		Amount:  1,
		Money:   1,
	}
	require.NoError(t, DB.Create(manualStatusOrder).Error)
	assert.True(t, errors.Is(ManualCompleteTopUp(manualStatusOrder.TradeNo), ErrTopUpOrderStatus))

	manualInvalidQuotaOrder := &TopUp{
		UserId:        user.Id,
		TradeNo:       "manual-invalid-quota-order",
		Status:        common.TopUpStatusPending,
		PaymentMethod: "stripe",
		Amount:        1,
		Money:         0,
	}
	require.NoError(t, DB.Create(manualInvalidQuotaOrder).Error)
	assert.True(t, errors.Is(ManualCompleteTopUp(manualInvalidQuotaOrder.TradeNo), ErrTopUpInvalidQuota))
}

func TestUserCheckinReturnsBusinessSentinels(t *testing.T) {
	truncateTables(t)
	user := createBusinessErrorTestUser(t, 0)
	setting := operation_setting.GetCheckinSetting()
	previous := *setting
	t.Cleanup(func() {
		*setting = previous
	})

	setting.Enabled = false
	_, err := UserCheckin(user.Id)
	assert.True(t, errors.Is(err, ErrCheckinDisabled))

	setting.Enabled = true
	setting.MinQuota = 10
	setting.MaxQuota = 10
	checkin, err := UserCheckin(user.Id)
	require.NoError(t, err)
	require.NotNil(t, checkin)
	assert.Equal(t, 10, checkin.QuotaAwarded)

	_, err = UserCheckin(user.Id)
	assert.True(t, errors.Is(err, ErrCheckinAlreadyToday))
}
