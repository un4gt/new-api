package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const (
	sessionRegistrationInviteIDKey        = "registration_invite_id"
	sessionRegistrationInviteExpiresAtKey = "registration_invite_expires_at"
)

type registrationInviteActivation struct {
	InviteID  int   `json:"invite_id"`
	ExpiresAt int64 `json:"expires_at"`
}

type activateRegistrationInviteRequest struct {
	Code string `json:"code"`
}

type createRegistrationInviteRequest struct {
	Note      string `json:"note"`
	ExpiresAt int64  `json:"expires_at"`
	MaxUses   int    `json:"max_uses"`
}

func readSessionInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		res, err := strconv.Atoi(typed)
		if err != nil {
			return 0, false
		}
		return res, true
	default:
		return 0, false
	}
}

func readSessionInt64(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	case string:
		res, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return 0, false
		}
		return res, true
	default:
		return 0, false
	}
}

func peekRegistrationInviteActivation(session sessions.Session) *registrationInviteActivation {
	inviteID, ok := readSessionInt(session.Get(sessionRegistrationInviteIDKey))
	if !ok || inviteID <= 0 {
		return nil
	}
	expiresAt, ok := readSessionInt64(session.Get(sessionRegistrationInviteExpiresAtKey))
	if !ok || expiresAt <= 0 {
		return nil
	}
	return &registrationInviteActivation{
		InviteID:  inviteID,
		ExpiresAt: expiresAt,
	}
}

func setRegistrationInviteActivation(session sessions.Session, activation *registrationInviteActivation) {
	if activation == nil {
		return
	}
	session.Set(sessionRegistrationInviteIDKey, activation.InviteID)
	session.Set(sessionRegistrationInviteExpiresAtKey, activation.ExpiresAt)
}

func clearRegistrationInviteActivation(session sessions.Session) {
	session.Delete(sessionRegistrationInviteIDKey)
	session.Delete(sessionRegistrationInviteExpiresAtKey)
}

func getRegistrationInviteActivation(c *gin.Context, session sessions.Session) (*registrationInviteActivation, error) {
	activation := peekRegistrationInviteActivation(session)
	if activation == nil {
		return nil, model.ErrRegistrationInviteRequired
	}
	if activation.ExpiresAt <= common.GetTimestamp() {
		clearRegistrationInviteActivation(session)
		_ = session.Save()
		return nil, model.ErrRegistrationInviteActivationExpired
	}
	return activation, nil
}

func shouldClearRegistrationInviteActivation(err error) bool {
	return errors.Is(err, model.ErrRegistrationInviteInvalid) ||
		errors.Is(err, model.ErrRegistrationInviteExpired) ||
		errors.Is(err, model.ErrRegistrationInviteUsed) ||
		errors.Is(err, model.ErrRegistrationInviteRevoked) ||
		errors.Is(err, model.ErrRegistrationInviteActivationExpired)
}

func writeRegistrationInviteError(c *gin.Context, session sessions.Session, err error) {
	if shouldClearRegistrationInviteActivation(err) {
		clearRegistrationInviteActivation(session)
		_ = session.Save()
	}

	switch {
	case errors.Is(err, model.ErrRegistrationInviteRequired):
		common.ApiErrorI18n(c, i18n.MsgRegistrationInviteRequired)
	case errors.Is(err, model.ErrRegistrationInviteCodeEmpty):
		common.ApiErrorI18n(c, i18n.MsgRegistrationInviteCodeEmpty)
	case errors.Is(err, model.ErrRegistrationInviteInvalid):
		common.ApiErrorI18n(c, i18n.MsgRegistrationInviteInvalid)
	case errors.Is(err, model.ErrRegistrationInviteExpired):
		common.ApiErrorI18n(c, i18n.MsgRegistrationInviteExpired)
	case errors.Is(err, model.ErrRegistrationInviteUsed):
		common.ApiErrorI18n(c, i18n.MsgRegistrationInviteUsed)
	case errors.Is(err, model.ErrRegistrationInviteRevoked):
		common.ApiErrorI18n(c, i18n.MsgRegistrationInviteRevoked)
	case errors.Is(err, model.ErrRegistrationInviteActivationExpired):
		common.ApiErrorI18n(c, i18n.MsgRegistrationInviteActivationExpired)
	case errors.Is(err, model.ErrRegistrationInviteNoteTooLong):
		common.ApiErrorI18n(c, i18n.MsgRegistrationInviteNoteTooLong)
	default:
		common.ApiError(c, err)
	}
}

func ActivateRegistrationInvite(c *gin.Context) {
	if !common.RegistrationInviteRequired {
		common.ApiSuccess(c, gin.H{
			"active":     false,
			"required":   false,
			"expires_at": int64(0),
		})
		return
	}

	var req activateRegistrationInviteRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	invite, err := model.GetRegistrationInviteByCode(req.Code)
	if err == nil {
		err = invite.ValidateAvailable(common.GetTimestamp())
	}
	if err != nil {
		writeRegistrationInviteError(c, sessions.Default(c), err)
		return
	}

	session := sessions.Default(c)
	expiresAt := common.GetTimestamp() + common.RegistrationInviteActivationTTLSeconds
	setRegistrationInviteActivation(session, &registrationInviteActivation{
		InviteID:  invite.Id,
		ExpiresAt: expiresAt,
	})
	if err := session.Save(); err != nil {
		common.ApiErrorI18n(c, i18n.MsgUserSessionSaveFailed)
		return
	}

	common.ApiSuccessI18n(c, i18n.MsgRegistrationInviteActivated, gin.H{
		"active":     true,
		"expires_at": expiresAt,
	})
}

func GetRegistrationInviteActivationStatus(c *gin.Context) {
	if !common.RegistrationInviteRequired {
		common.ApiSuccess(c, gin.H{
			"active":     false,
			"required":   false,
			"expires_at": int64(0),
		})
		return
	}

	session := sessions.Default(c)
	activation := peekRegistrationInviteActivation(session)
	if activation == nil || activation.ExpiresAt <= common.GetTimestamp() {
		if activation != nil {
			clearRegistrationInviteActivation(session)
			_ = session.Save()
		}
		common.ApiSuccess(c, gin.H{
			"active":     false,
			"required":   true,
			"expires_at": int64(0),
		})
		return
	}

	invite, err := model.GetRegistrationInviteById(activation.InviteID)
	if err != nil || invite.ValidateAvailable(common.GetTimestamp()) != nil {
		clearRegistrationInviteActivation(session)
		_ = session.Save()
		common.ApiSuccess(c, gin.H{
			"active":     false,
			"required":   true,
			"expires_at": int64(0),
		})
		return
	}

	common.ApiSuccess(c, gin.H{
		"active":     true,
		"required":   true,
		"expires_at": activation.ExpiresAt,
	})
}

func GetRegistrationInvites(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	invites, total, err := model.GetRegistrationInvites(pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(invites)
	common.ApiSuccess(c, pageInfo)
}

func CreateRegistrationInvite(c *gin.Context) {
	var req createRegistrationInviteRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	req.Note = strings.TrimSpace(req.Note)
	if req.ExpiresAt != 0 && req.ExpiresAt <= common.GetTimestamp() {
		common.ApiErrorI18n(c, i18n.MsgInvalidParams)
		return
	}

	invite, code, err := model.CreateRegistrationInvite(c.GetInt("id"), req.Note, req.ExpiresAt, req.MaxUses)
	if err != nil {
		writeRegistrationInviteError(c, sessions.Default(c), err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"invite":      invite,
			"invite_code": code,
		},
	})
}

func RevokeRegistrationInvite(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorI18n(c, i18n.MsgInvalidId)
		return
	}

	if err := model.RevokeRegistrationInviteById(id); err != nil {
		writeRegistrationInviteError(c, sessions.Default(c), err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
