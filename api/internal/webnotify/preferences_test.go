package webnotify

import (
	"testing"
	"time"
)

func TestNotificationPriorityAndModes(t *testing.T) {
	now := time.Date(2026, 1, 6, 8, 0, 0, 0, time.UTC)
	p := defaults()
	p.DNDUntil = &[]time.Time{now.Add(time.Hour)}[0]
	d := Evaluate(p, "silent", nil, nil, "message", "personal", now)
	if !d.Visual || d.Push || d.MessageSound {
		t.Fatalf("DND should suppress configured push and sound but preserve visual notices: %+v", d)
	}
	p.DNDUntil = nil
	d = Evaluate(p, "off", &[]time.Time{now.Add(time.Hour)}[0], nil, "message", "personal", now)
	if d.Visual || d.Push {
		t.Fatalf("timed chat mute should suppress alerts: %+v", d)
	}
	d = Evaluate(p, "off", &[]time.Time{now.Add(-time.Second)}[0], nil, "message", "personal", now)
	if !d.Visual || !d.Push {
		t.Fatalf("expired mute should restore all notifications: %+v", d)
	}
	d = Evaluate(p, "mentions", nil, nil, "message", "group", now)
	if d.Visual || d.Push {
		t.Fatalf("mentions-only chat should suppress regular message alerts: %+v", d)
	}
	d = Evaluate(p, "mentions", nil, nil, "mention", "group", now)
	if !d.Visual || !d.Push || !d.Mention {
		t.Fatalf("mentions-only chat should preserve mention alert: %+v", d)
	}
}

func TestQuietScheduleHandlesWeekBoundaryAndTimezone(t *testing.T) {
	p := defaults()
	p.QuietEnabled = true
	p.QuietDays = 1 << 0
	p.QuietStart = "23:00"
	p.QuietEnd = "09:00"
	p.Timezone = "Asia/Qyzylorda"
	// Tuesday 05:00 local is still inside Monday's overnight window.
	if !quietNow(p, time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("overnight quiet window lost Monday schedule after midnight")
	}
	if quietNow(p, time.Date(2025, 1, 7, 6, 0, 0, 0, time.UTC)) {
		t.Fatal("quiet window remained active after local end")
	}
}
