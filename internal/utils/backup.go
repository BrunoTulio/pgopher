package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func IsFileBackup(name string) bool {
	if !strings.HasSuffix(name, ".age") && !strings.HasSuffix(name, ".gz") {
		return false
	}

	return true
}

func GenerateShortID(name string, modTime time.Time) string {
	data := fmt.Sprintf("%s-%d", name, modTime.Unix())
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:4])
}
