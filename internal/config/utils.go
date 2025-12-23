package config

import (
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

	_ = os.Unsetenv(key)

	return value
}

func stringOrEmpty(key string, defaultValue string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	_ = os.Unsetenv(key)
	return value
}

func stringLookup(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	_ = os.Unsetenv(key)
	return value, true
}

func intOrEmpty(key string, defaultValue int) int {
	valueStr, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	_ = os.Unsetenv(key)

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func intLookup(key string) (int, bool) {
	valueStr, ok := os.LookupEnv(key)
	if !ok {
		return 0, false
	}
	_ = os.Unsetenv(key)
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, false
	}

	return value, true
}

func mustBool(key string) bool {
	valueStr, ok := os.LookupEnv(key)
	if !ok {
		panic("Config error: environment variable not found: " + key)
	}

	_ = os.Unsetenv(key)
	value, err := strconv.ParseBool(valueStr)

	if err != nil {
		panic("Config error parse bool: " + key + ": " + valueStr)
	}

	return value
}

func boolOrEmpty(key string, defaultValue bool) bool {
	valueStr, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	_ = os.Unsetenv(key)

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func boolLookup(key string) (bool, bool) {
	valueStr, ok := os.LookupEnv(key)
	if !ok {
		return false, false
	}
	_ = os.Unsetenv(key)

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return false, false
	}

	return value, true
}
func stringsMust(key string) []string {
	valueStr, ok := os.LookupEnv(key)
	if !ok {
		panic("Config error: environment variable not found: " + key)
	}
	_ = os.Unsetenv(key)
	return strings.Split(valueStr, ",")
}

func stringsOrEmpty(key string, defaultValue []string) []string {
	valueStr, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	_ = os.Unsetenv(key)
	return strings.Split(valueStr, ",")
}

func stringsLookup(key string) ([]string, bool) {
	valueStr, ok := os.LookupEnv(key)
	if !ok {
		return nil, false
	}
	_ = os.Unsetenv(key)
	return strings.Split(valueStr, ","), true
}
