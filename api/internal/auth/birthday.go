package auth

import (
	"errors"
	"time"
)

var ErrBirthDate = errors.New("invalid birth date")

func ParseBirthDate(value string, now time.Time) (time.Time, error) {
	d, err := time.Parse("2006-01-02", value)
	if err != nil || d.Format("2006-01-02") != value || d.Year() < 1900 {
		return time.Time{}, ErrBirthDate
	}
	today := now.In(time.UTC)
	dateToday := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	if !d.Before(dateToday) {
		return time.Time{}, ErrBirthDate
	}
	age := dateToday.Year() - d.Year()
	if dateToday.Month() < d.Month() || dateToday.Month() == d.Month() && dateToday.Day() < d.Day() {
		age--
	}
	if age < 14 {
		return time.Time{}, ErrBirthDate
	}
	return d, nil
}
