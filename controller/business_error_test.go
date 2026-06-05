package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

func newBusinessErrorTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return ctx, recorder
}

func decodeBusinessErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var response map[string]any
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return response
}

func TestWriteRedeemErrorMapsSentinelsToRedemptionKeys(t *testing.T) {
	tests := []struct {
		name string
		err  error
		key  string
	}{
		{name: "not provided", err: model.ErrRedemptionNotProvided, key: i18n.MsgRedemptionNotProvided},
		{name: "invalid", err: model.ErrRedemptionInvalid, key: i18n.MsgRedemptionInvalid},
		{name: "used", err: model.ErrRedemptionUsed, key: i18n.MsgRedemptionUsed},
		{name: "expired", err: model.ErrRedemptionExpired, key: i18n.MsgRedemptionExpired},
		{name: "failed", err: fmt.Errorf("%w: save redemption", model.ErrRedeemFailed), key: i18n.MsgRedemptionFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, recorder := newBusinessErrorTestContext(t)

			writeRedeemError(ctx, tt.err)

			response := decodeBusinessErrorResponse(t, recorder)
			if response["success"] != false {
				t.Fatalf("expected failure response, got %#v", response)
			}
			if response["message"] != tt.key {
				t.Fatalf("expected message key %q, got %#v", tt.key, response["message"])
			}
		})
	}
}

func TestWriteCheckinErrorMapsSentinelsToCheckinKeys(t *testing.T) {
	tests := []struct {
		name string
		err  error
		key  string
	}{
		{name: "disabled", err: model.ErrCheckinDisabled, key: i18n.MsgCheckinDisabled},
		{name: "already today", err: model.ErrCheckinAlreadyToday, key: i18n.MsgCheckinAlreadyToday},
		{name: "failed", err: fmt.Errorf("%w: create record", model.ErrCheckinFailed), key: i18n.MsgCheckinFailed},
		{name: "quota failed", err: fmt.Errorf("%w: update quota", model.ErrCheckinQuotaFailed), key: i18n.MsgCheckinQuotaFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, recorder := newBusinessErrorTestContext(t)

			writeCheckinError(ctx, tt.err)

			response := decodeBusinessErrorResponse(t, recorder)
			if response["success"] != false {
				t.Fatalf("expected failure response, got %#v", response)
			}
			if response["message"] != tt.key {
				t.Fatalf("expected message key %q, got %#v", tt.key, response["message"])
			}
		})
	}
}

func TestWriteCheckinErrorFallsBackForUnknownError(t *testing.T) {
	ctx, recorder := newBusinessErrorTestContext(t)
	err := errors.New("unexpected visible error")

	writeCheckinError(ctx, err)

	response := decodeBusinessErrorResponse(t, recorder)
	if response["success"] != false {
		t.Fatalf("expected failure response, got %#v", response)
	}
	if response["message"] != err.Error() {
		t.Fatalf("expected fallback message %q, got %#v", err.Error(), response["message"])
	}
}

func TestGetCheckinStatusDisabledUsesI18nKey(t *testing.T) {
	setting := operation_setting.GetCheckinSetting()
	previous := *setting
	t.Cleanup(func() {
		*setting = previous
	})
	setting.Enabled = false

	ctx, recorder := newBusinessErrorTestContext(t)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/user/checkin/status", nil)

	GetCheckinStatus(ctx)

	response := decodeBusinessErrorResponse(t, recorder)
	if response["success"] != false {
		t.Fatalf("expected failure response, got %#v", response)
	}
	if response["message"] != i18n.MsgCheckinDisabled {
		t.Fatalf("expected disabled message key %q, got %#v", i18n.MsgCheckinDisabled, response["message"])
	}
}
