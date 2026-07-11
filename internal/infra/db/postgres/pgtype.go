package postgres

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// TimestampzToTimePtr converts a nullable pgtype.Timestamptz to *time.Time.
func TimestampzToTimePtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}

	t := ts.Time
	return &t
}

// TimeToTimestampz wraps a time.Time into a valid pgtype.Timestamptz for a query parameter.
func TimeToTimestampz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// DateToTime converts a non-null Postgres date to a UTC-midnight time.
func DateToTime(d pgtype.Date) time.Time {
	return time.Date(d.Time.Year(), d.Time.Month(), d.Time.Day(), 0, 0, 0, 0, time.UTC)
}

// TimeToDate wraps the calendar date portion of t into a valid pgtype.Date.
func TimeToDate(t time.Time) pgtype.Date {
	return pgtype.Date{Time: time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}
