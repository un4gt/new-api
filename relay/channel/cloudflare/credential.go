package cloudflare

import (
	"errors"
	"fmt"
	"strings"
)

var errInvalidCredentialFormat = errors.New("Cloudflare credential must use ACCOUNT_ID,API_TOKEN format")

// ParseCredential parses one logical Cloudflare credential. Multi-key values
// must be split into individual lines before calling this function.
func ParseCredential(value string) (accountID string, apiToken string, err error) {
	if strings.ContainsAny(value, "\r\n") || strings.Count(value, ",") != 1 {
		return "", "", errInvalidCredentialFormat
	}

	parts := strings.SplitN(value, ",", 2)
	accountID = strings.TrimSpace(parts[0])
	apiToken = strings.TrimSpace(parts[1])
	if accountID == "" || apiToken == "" {
		return "", "", errInvalidCredentialFormat
	}
	return accountID, apiToken, nil
}

// NormalizeCredentialList validates and normalizes newline-delimited
// Cloudflare credentials. Empty lines are ignored to match the existing
// multi-key channel workflow.
func NormalizeCredentialList(value string) ([]string, error) {
	credentials := make([]string, 0)
	for index, line := range strings.Split(value, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		accountID, apiToken, err := ParseCredential(line)
		if err != nil {
			return nil, fmt.Errorf("invalid Cloudflare credential on line %d: %w", index+1, err)
		}
		credentials = append(credentials, accountID+","+apiToken)
	}
	if len(credentials) == 0 {
		return nil, errInvalidCredentialFormat
	}
	return credentials, nil
}

// ValidateCredentialList validates newline-delimited Cloudflare credentials.
func ValidateCredentialList(value string) error {
	_, err := NormalizeCredentialList(value)
	if err != nil {
		return err
	}
	return nil
}
