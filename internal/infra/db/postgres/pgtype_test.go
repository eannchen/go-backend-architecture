package postgres

import (
	"testing"
	"time"
)

func TestTimeToDate_UsesOriginalCalendarDate(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Taipei")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	localMidnight := time.Date(2026, 5, 7, 0, 0, 0, 0, loc)
	got := TimeToDate(localMidnight)
	if !got.Valid {
		t.Fatal("TimeToDate() returned invalid date")
	}

	// Preserve a user's calendar date instead of shifting it during UTC conversion.
	want := time.Date(2026, 5, 7, 0, 0, 0, 0, time.UTC)
	if !got.Time.Equal(want) {
		t.Fatalf("TimeToDate() = %s, want %s", got.Time, want)
	}
}
