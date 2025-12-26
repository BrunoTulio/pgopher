package utils

import "time"

var (
	configuredLocation *time.Location
	configuredFormat   string = "2006-01-02 15:04:05"
)

func InitTimezone(loc *time.Location, format string) {
	configuredLocation = loc
	configuredFormat = format
}

func FormatTime(t time.Time) string {
	if configuredLocation == nil {
		return t.Format(configuredFormat)
	}
	return t.In(configuredLocation).Format(configuredFormat)
}
