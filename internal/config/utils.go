package config

import (
	"encoding/base64"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func remoteNames() []string {
	re := regexp.MustCompile(`^REMOTE_([A-Z0-9_]+)_[A-Z0-9_]+$`)
	set := map[string]struct{}{}

	for _, env := range os.Environ() {
		key := strings.SplitN(env, "=", 2)[0]

		m := re.FindStringSubmatch(key)
		if len(m) == 2 {
			set[m[1]] = struct{}{}
		}
	}

	var names []string
	for name := range set {
		names = append(names, name)
	}
	return names
}

func mustString(key string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		panic("Config error: environment variable not found: " + key)
	}
	return value
}

func stringOrEmpty(key string, defaultValue string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	return value
}

func intOrEmpty(key string, defaultValue int) int {
	valueStr, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func mustInt64(key string) int64 {
	valueStr := mustString(key)
	value, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		panic("Config error: invalid int64 value for " + key + ": " + valueStr)
	}
	return value
}

func boolOrEmpty(key string, defaultValue bool) bool {
	valueStr, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func stringsOrEmpty(key string, defaultValue []string) []string {
	valueStr, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	return strings.Split(valueStr, ",")
}

func decodeBase64(value string) string {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		// se der erro, volta vazio ou loga
		return ""
	}
	return string(data)
}
