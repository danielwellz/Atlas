package food

import (
	"fmt"
	"regexp"
	"strings"
)

var upcCodePattern = regexp.MustCompile(`^\d{8,14}$`)

func NormalizeUPC(code string) (string, error) {
	normalized := strings.TrimSpace(code)
	normalized = strings.ReplaceAll(normalized, " ", "")
	normalized = strings.ReplaceAll(normalized, "-", "")
	if !upcCodePattern.MatchString(normalized) {
		return "", fmt.Errorf("invalid upc code")
	}
	return normalized, nil
}
