package util

import "time"

// ToFacilityTime converts a UTC time to the facility's local timezone.
func ToFacilityTime(t time.Time, tz *time.Location) time.Time {
	return t.In(tz)
}

// FormatFacilityTime formats a UTC time in the facility timezone.
func FormatFacilityTime(t time.Time, tz *time.Location) string {
	return t.In(tz).Format(time.RFC3339)
}

// NowUTC returns current time in UTC.
func NowUTC() time.Time {
	return time.Now().UTC()
}
