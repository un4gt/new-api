package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	registrationInviteCodeLength  = 24
	registrationInviteCodeGroup   = 4
	RegistrationInviteNoteMaxSize = 255
)

var (
	ErrRegistrationInviteRequired          = errors.New("registration invite required")
	ErrRegistrationInviteCodeEmpty         = errors.New("registration invite code empty")
	ErrRegistrationInviteInvalid           = errors.New("registration invite invalid")
	ErrRegistrationInviteExpired           = errors.New("registration invite expired")
	ErrRegistrationInviteUsed              = errors.New("registration invite used")
	ErrRegistrationInviteRevoked           = errors.New("registration invite revoked")
	ErrRegistrationInviteActivationExpired = errors.New("registration invite activation expired")
	ErrRegistrationInviteNoteTooLong       = errors.New("registration invite note too long")
)

type RegistrationInvite struct {
	Id                 int            `json:"id" gorm:"primaryKey"`
	CodeHash           string         `json:"-" gorm:"type:char(64);uniqueIndex"`
	Status             string         `json:"status" gorm:"type:varchar(16);not null;default:'active';index"`
	Note               string         `json:"note" gorm:"type:varchar(255)"`
	CreatedBy          int            `json:"created_by" gorm:"index"`
	UsedBy             int            `json:"used_by" gorm:"index"`
	CreatedAt          int64          `json:"created_at" gorm:"autoCreateTime"`
	UsedAt             int64          `json:"used_at"`
	ExpiresAt          int64          `json:"expires_at" gorm:"index"`
	MaxUses            int            `json:"max_uses" gorm:"not null;default:1"`
	UseCount           int            `json:"use_count" gorm:"not null;default:0"`
	UsedProvider       string         `json:"used_provider" gorm:"type:varchar(32)"`
	UsedProviderUserID string         `json:"used_provider_user_id" gorm:"type:varchar(128)"`
	DeletedAt          gorm.DeletedAt `json:"-" gorm:"index"`
}

func (RegistrationInvite) TableName() string {
	return "registration_invites"
}

func normalizeRegistrationInviteCode(code string) string {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	return normalized
}

func hashRegistrationInviteCode(code string) string {
	sum := sha256.Sum256([]byte(normalizeRegistrationInviteCode(code)))
	return hex.EncodeToString(sum[:])
}

func formatRegistrationInviteCode(code string) string {
	normalized := normalizeRegistrationInviteCode(code)
	if len(normalized) <= registrationInviteCodeGroup {
		return normalized
	}

	var builder strings.Builder
	for idx, ch := range normalized {
		if idx > 0 && idx%registrationInviteCodeGroup == 0 {
			builder.WriteByte('-')
		}
		builder.WriteRune(ch)
	}
	return builder.String()
}

func generateRegistrationInviteCode() (string, string, error) {
	code, err := common.GenerateRandomCharsKey(registrationInviteCodeLength)
	if err != nil {
		return "", "", err
	}

	formatted := formatRegistrationInviteCode(code)
	return formatted, hashRegistrationInviteCode(formatted), nil
}

func (invite *RegistrationInvite) ValidateAvailable(now int64) error {
	if invite == nil || invite.Id == 0 {
		return ErrRegistrationInviteInvalid
	}
	if invite.Status == common.RegistrationInviteStatusRevoked {
		return ErrRegistrationInviteRevoked
	}
	if invite.ExpiresAt != 0 && invite.ExpiresAt <= now {
		return ErrRegistrationInviteExpired
	}
	if invite.Status == common.RegistrationInviteStatusUsed {
		return ErrRegistrationInviteUsed
	}
	if invite.MaxUses > 0 && invite.UseCount >= invite.MaxUses {
		return ErrRegistrationInviteUsed
	}
	if invite.Status != common.RegistrationInviteStatusActive {
		return ErrRegistrationInviteInvalid
	}
	return nil
}

func CreateRegistrationInvite(createdBy int, note string, expiresAt int64, maxUses int) (*RegistrationInvite, string, error) {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > RegistrationInviteNoteMaxSize {
		return nil, "", ErrRegistrationInviteNoteTooLong
	}
	if maxUses <= 0 {
		maxUses = 1
	}

	for idx := 0; idx < 5; idx++ {
		code, hash, err := generateRegistrationInviteCode()
		if err != nil {
			return nil, "", err
		}

		invite := &RegistrationInvite{
			CodeHash:  hash,
			Status:    common.RegistrationInviteStatusActive,
			Note:      note,
			CreatedBy: createdBy,
			ExpiresAt: expiresAt,
			MaxUses:   maxUses,
		}
		if err := DB.Create(invite).Error; err != nil {
			lowerErr := strings.ToLower(err.Error())
			if strings.Contains(lowerErr, "duplicate") || strings.Contains(lowerErr, "unique") {
				continue
			}
			return nil, "", err
		}
		return invite, code, nil
	}

	return nil, "", fmt.Errorf("failed to generate unique registration invite code")
}

func GetRegistrationInviteById(id int) (*RegistrationInvite, error) {
	if id <= 0 {
		return nil, ErrRegistrationInviteInvalid
	}

	var invite RegistrationInvite
	if err := DB.First(&invite, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRegistrationInviteInvalid
		}
		return nil, err
	}
	return &invite, nil
}

func GetRegistrationInviteByCode(code string) (*RegistrationInvite, error) {
	normalized := normalizeRegistrationInviteCode(code)
	if normalized == "" {
		return nil, ErrRegistrationInviteCodeEmpty
	}

	var invite RegistrationInvite
	if err := DB.Where("code_hash = ?", hashRegistrationInviteCode(normalized)).First(&invite).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRegistrationInviteInvalid
		}
		return nil, err
	}
	return &invite, nil
}

func GetRegistrationInvites(pageInfo *common.PageInfo) (invites []*RegistrationInvite, total int64, err error) {
	query := DB.Model(&RegistrationInvite{})

	err = query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&invites).Error
	if err != nil {
		return nil, 0, err
	}

	return invites, total, nil
}

func RevokeRegistrationInviteById(id int) error {
	invite, err := GetRegistrationInviteById(id)
	if err != nil {
		return err
	}
	if invite.Status == common.RegistrationInviteStatusRevoked {
		return ErrRegistrationInviteRevoked
	}
	if invite.Status == common.RegistrationInviteStatusUsed || (invite.MaxUses > 0 && invite.UseCount >= invite.MaxUses) {
		return ErrRegistrationInviteUsed
	}

	return DB.Model(&RegistrationInvite{}).Where("id = ?", id).Update("status", common.RegistrationInviteStatusRevoked).Error
}

func ConsumeRegistrationInviteTx(tx *gorm.DB, inviteID int, userID int, provider string, providerUserID string) error {
	if inviteID <= 0 {
		return ErrRegistrationInviteRequired
	}

	var invite RegistrationInvite
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", inviteID).First(&invite).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRegistrationInviteInvalid
		}
		return err
	}

	now := common.GetTimestamp()
	if err := invite.ValidateAvailable(now); err != nil {
		return err
	}

	nextUseCount := invite.UseCount + 1
	nextStatus := common.RegistrationInviteStatusActive
	if invite.MaxUses <= 0 || nextUseCount >= invite.MaxUses {
		nextStatus = common.RegistrationInviteStatusUsed
	}

	return tx.Model(&RegistrationInvite{}).Where("id = ?", invite.Id).Updates(map[string]any{
		"use_count":             nextUseCount,
		"status":                nextStatus,
		"used_by":               userID,
		"used_at":               now,
		"used_provider":         provider,
		"used_provider_user_id": providerUserID,
	}).Error
}
