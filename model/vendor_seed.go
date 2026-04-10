package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

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

// EnsureDefaultVendors inserts a minimal set of commonly used vendors into the vendors table
// if they do not exist yet. This is used to keep the model metadata UI usable out of the box.
//
// It only INSERTs missing vendors and never updates existing ones.
func EnsureDefaultVendors() error {
	if DB == nil {
		return nil
	}

	defaultNames := make(map[string]struct{}, len(defaultVendorRules)+1)
	for _, vendorName := range defaultVendorRules {
		name := strings.TrimSpace(vendorName)
		if name == "" {
			continue
		}
		defaultNames[name] = struct{}{}
	}
	// Additional vendor for new channels that are not covered by defaultVendorRules.
	defaultNames["Elastic"] = struct{}{}

	type nameRow struct {
		Name string
	}
	var rows []nameRow
	if err := DB.Model(&Vendor{}).Select("name").Find(&rows).Error; err != nil {
		return err
	}

	existing := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		name := strings.TrimSpace(r.Name)
		if name == "" {
			continue
		}
		existing[name] = struct{}{}
	}

	created := 0
	for name := range defaultNames {
		if _, ok := existing[name]; ok {
			continue
		}
		v := &Vendor{
			Name:   name,
			Status: 1,
			Icon:   getDefaultVendorIcon(name),
		}
		if err := v.Insert(); err != nil {
			if isDuplicateKeyError(err) {
				continue
			}
			return err
		}
		created++
		existing[name] = struct{}{}
	}

	if created > 0 {
		common.SysLog(fmt.Sprintf("Seeded %d default vendors", created))
	}
	return nil
}
