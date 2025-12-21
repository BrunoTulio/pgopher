package utils

import "strings"

func IsFileBackup(name string) bool {
	if !strings.HasSuffix(name, ".age") && !strings.HasSuffix(name, ".gz") {
		return false
	}

	return true
}
