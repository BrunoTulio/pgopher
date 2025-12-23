package utils

import "os"

func DirExists(path string) bool {
	info, err := os.Stat(path)

	if err != nil {
		return false
	}
	return info.IsDir()
}
