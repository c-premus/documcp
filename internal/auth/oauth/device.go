package oauth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// Base-20 character set excluding vowels and confusing characters (0, 1, O, I).
const deviceCodeCharset = "BCDFGHJKLMNPQRSTVWXZ"

// GenerateUserCode produces an XXXX-XXXX user code from the base-20 charset.
func GenerateUserCode() (string, error) {
	code := make([]byte, 8)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(deviceCodeCharset))))
		if err != nil {
			return "", fmt.Errorf("generating user code: %w", err)
		}
		code[i] = deviceCodeCharset[n.Int64()]
	}
	return string(code[:4]) + "-" + string(code[4:]), nil
}

// NormalizeUserCode removes dashes and uppercases for comparison.
func NormalizeUserCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(code, "-", ""))
}
