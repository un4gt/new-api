package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// NOTE: Error translation is not guaranteed to be enabled in GORM config,
	// so we also fallback to string matching for PostgreSQL unique violations.
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "duplicate key") ||
		strings.Contains(lower, "violates unique constraint") ||
		strings.Contains(lower, "duplicated key")
}
