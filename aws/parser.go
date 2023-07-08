package aws

import (
	"strconv"
	"strings"
)

func ParseBool(s string) bool {
	return strings.ToLower(s) == "yes"
}

func ParseInt(s string, defaultValue int64) int64 {
	if s == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func ParseFloat(s string, defaultValue float64) float64 {
	if s == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func ParseClockSpeed(speed string) Range[float64] {
	if speed == "" {
		return Range[float64]{}
	}

	speed = strings.ToLower(speed)

	parts := strings.Split(speed, " ")

	rng := Range[float64]{
		Unit: parts[len(parts)-1],
		Min:  -1,
	}

	value := ParseFloat(parts[len(parts)-2], 0)

	rng.Max = value

	if parts[0] == "up" && parts[1] == "to" {
		rng.Min = 0
	} else {
		rng.Min = value
	}

	return rng
}

func ParseMemoryMib(memory string) int64 {
	if memory == "" {
		return 0
	}

	memory = strings.ToLower(memory)

	parts := strings.Split(memory, " ")

	// Defaults to GiB
	multiplier := 1024.0
	unit := parts[len(parts)-1]
	if unit == "mib" {
		multiplier = 1.0
	}

	value := ParseFloat(parts[0], 0)

	return int64(value * multiplier)
}
