package utils

import "encoding/base64"

func DecodeBase64(value string) string {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		// se der erro, volta vazio ou loga
		return ""
	}
	return string(data)
}
