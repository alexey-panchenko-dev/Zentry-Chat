package auth

import (
	"errors"
	"time"
)

var ErrBirthDate = errors.New("invalid birth date")

func ParseBirthDate(value string, now time.Time) (time.Time, error) {
	d, err := time.Parse("2006-01-02", value)
	if err != nil || d.Format("2006-01-02") != value || d.Year() < 1900 || value > now.Format("2006-01-02") {
		return time.Time{}, ErrBirthDate
	}
	return d, nil
}
