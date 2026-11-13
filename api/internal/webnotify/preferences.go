package webnotify

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrInvalidPreference = errors.New("invalid notification preference")
var ErrChatUnavailable = errors.New("chat unavailable")

type Preferences struct {
	SoundMessages    bool       `json:"sound_messages"`
	PushEnabled      bool       `json:"push_enabled"`
	SoundCalls       bool       `json:"sound_calls"`
	MentionsEnabled  bool       `json:"mentions_enabled"`
	DNDUntil         *time.Time `json:"dnd_until"`
	DNDDuration      string     `json:"dnd_duration,omitempty"`
	MuteMessageSound bool       `json:"dnd_mute_message_sound"`
	SuppressPush     bool       `json:"dnd_suppress_push"`
	MuteCallSound    bool       `json:"dnd_mute_call_sound"`
	SuppressMentions bool       `json:"dnd_suppress_mentions"`
	QuietEnabled     bool       `json:"quiet_enabled"`
	QuietDays        int        `json:"quiet_days"`
	QuietStart       string     `json:"quiet_start"`
	QuietEnd         string     `json:"quiet_end"`
	Timezone         string     `json:"timezone"`
}

type ChatPreference struct {
	Scope      string     `json:"scope"`
	ChatID     string     `json:"chat_id"`
	Mode       string     `json:"mode"`
	MutedUntil *time.Time `json:"muted_until"`
}
type Exception struct {
	ContactID      string `json:"contact_id"`
	AllowDNDPush   bool   `json:"allow_dnd_push"`
	AllowCallSound bool   `json:"allow_call_sound"`
	Important      bool   `json:"important"`
}
type Settings struct {
	Global     Preferences      `json:"global"`
	Chats      []ChatPreference `json:"chats"`
	Exceptions []Exception      `json:"exceptions"`
}

func defaults() Preferences {
	return Preferences{SoundMessages: true, PushEnabled: true, SoundCalls: true, MentionsEnabled: true, MuteMessageSound: true, SuppressPush: true, MuteCallSound: true, QuietDays: 127, QuietStart: "23:00", QuietEnd: "09:00", Timezone: "UTC"}
}

func (s *Service) GetPreferences(ctx context.Context, user string) (Settings, error) {
	out := Settings{Global: defaults(), Chats: []ChatPreference{}, Exceptions: []Exception{}}
	err := s.DB.QueryRow(ctx, `SELECT sound_messages,push_enabled,sound_calls,mentions_enabled,dnd_until,dnd_mute_message_sound,dnd_suppress_push,dnd_mute_call_sound,dnd_suppress_mentions,quiet_enabled,quiet_days,quiet_start::text,quiet_end::text,timezone FROM notification_preferences WHERE user_id=$1`, user).Scan(&out.Global.SoundMessages, &out.Global.PushEnabled, &out.Global.SoundCalls, &out.Global.MentionsEnabled, &out.Global.DNDUntil, &out.Global.MuteMessageSound, &out.Global.SuppressPush, &out.Global.MuteCallSound, &out.Global.SuppressMentions, &out.Global.QuietEnabled, &out.Global.QuietDays, &out.Global.QuietStart, &out.Global.QuietEnd, &out.Global.Timezone)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return out, err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	rows, err := s.DB.Query(ctx, `SELECT scope,chat_id::text,mode,muted_until FROM chat_notification_preferences WHERE user_id=$1`, user)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var v ChatPreference
		if err = rows.Scan(&v.Scope, &v.ChatID, &v.Mode, &v.MutedUntil); err != nil {
			return out, err
		}
		out.Chats = append(out.Chats, v)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	rows, err = s.DB.Query(ctx, `SELECT contact_id::text,allow_dnd_push,allow_call_sound,important FROM notification_exceptions WHERE user_id=$1`, user)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var v Exception
		if err = rows.Scan(&v.ContactID, &v.AllowDNDPush, &v.AllowCallSound, &v.Important); err != nil {
			return out, err
		}
		out.Exceptions = append(out.Exceptions, v)
	}
	return out, rows.Err()
}

func (s *Service) SavePreferences(ctx context.Context, user string, v Preferences) error {
	if v.QuietDays < 0 || v.QuietDays > 127 || len(v.Timezone) > 100 {
		return ErrInvalidPreference
	}
	if _, err := time.LoadLocation(v.Timezone); err != nil {
		return ErrInvalidPreference
	}
	if _, err := time.Parse("15:04", v.QuietStart); err != nil {
		return ErrInvalidPreference
	}
	if _, err := time.Parse("15:04", v.QuietEnd); err != nil {
		return ErrInvalidPreference
	}
	if v.DNDDuration != "" {
		now := time.Now().UTC()
		switch v.DNDDuration {
		case "off":
			v.DNDUntil = nil
		case "1h":
			t := now.Add(time.Hour)
			v.DNDUntil = &t
		case "8h":
			t := now.Add(8 * time.Hour)
			v.DNDUntil = &t
		case "1d":
			t := now.Add(24 * time.Hour)
			v.DNDUntil = &t
		case "1w":
			t := now.Add(7 * 24 * time.Hour)
			v.DNDUntil = &t
		case "forever":
			t := now.AddDate(100, 0, 0)
			v.DNDUntil = &t
		default:
			return ErrInvalidPreference
		}
	}
	_, err := s.DB.Exec(ctx, `INSERT INTO notification_preferences(user_id,sound_messages,push_enabled,sound_calls,mentions_enabled,dnd_until,dnd_mute_message_sound,dnd_suppress_push,dnd_mute_call_sound,dnd_suppress_mentions,quiet_enabled,quiet_days,quiet_start,quiet_end,timezone) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::time,$14::time,$15) ON CONFLICT(user_id) DO UPDATE SET sound_messages=EXCLUDED.sound_messages,push_enabled=EXCLUDED.push_enabled,sound_calls=EXCLUDED.sound_calls,mentions_enabled=EXCLUDED.mentions_enabled,dnd_until=EXCLUDED.dnd_until,dnd_mute_message_sound=EXCLUDED.dnd_mute_message_sound,dnd_suppress_push=EXCLUDED.dnd_suppress_push,dnd_mute_call_sound=EXCLUDED.dnd_mute_call_sound,dnd_suppress_mentions=EXCLUDED.dnd_suppress_mentions,quiet_enabled=EXCLUDED.quiet_enabled,quiet_days=EXCLUDED.quiet_days,quiet_start=EXCLUDED.quiet_start,quiet_end=EXCLUDED.quiet_end,timezone=EXCLUDED.timezone,updated_at=now()`, user, v.SoundMessages, v.PushEnabled, v.SoundCalls, v.MentionsEnabled, v.DNDUntil, v.MuteMessageSound, v.SuppressPush, v.MuteCallSound, v.SuppressMentions, v.QuietEnabled, v.QuietDays, v.QuietStart, v.QuietEnd, v.Timezone)
	return err
}

func (s *Service) SetChatPreference(ctx context.Context, user, scope, id, mode string, duration string) error {
	if (scope != "personal" && scope != "group") || (mode != "all" && mode != "silent" && mode != "mentions" && mode != "off") {
		return ErrInvalidPreference
	}
	if scope == "personal" {
		var ok bool
		err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2)`, id, user).Scan(&ok)
		if err != nil {
			return err
		}
		if !ok {
			return ErrChatUnavailable
		}
	} else {
		var ok bool
		err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id=$1 AND user_id=$2)`, id, user).Scan(&ok)
		if err != nil {
			return err
		}
		if !ok {
			return ErrChatUnavailable
		}
	}
	var until *time.Time
	now := time.Now().UTC()
	switch duration {
	case "":
	case "1h":
		t := now.Add(time.Hour)
		until = &t
		mode = "off"
	case "8h":
		t := now.Add(8 * time.Hour)
		until = &t
		mode = "off"
	case "1w":
		t := now.AddDate(0, 0, 7)
		until = &t
		mode = "off"
	case "forever":
	case "off":
		mode = "all"
	default:
		return ErrInvalidPreference
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO chat_notification_preferences(user_id,scope,chat_id,mode,muted_until) VALUES($1,$2,$3,$4,$5) ON CONFLICT(user_id,scope,chat_id) DO UPDATE SET mode=EXCLUDED.mode,muted_until=EXCLUDED.muted_until,updated_at=now()`, user, scope, id, mode, until); err != nil {
		return err
	}
	return nil
}

func (s *Service) SetException(ctx context.Context, user, contact string, v Exception) error {
	if user == contact {
		return ErrInvalidPreference
	}
	var connected bool
	err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM conversations c JOIN conversation_members a ON a.conversation_id=c.id AND a.user_id=$1 JOIN conversation_members b ON b.conversation_id=c.id AND b.user_id=$2)`, user, contact).Scan(&connected)
	if err != nil {
		return err
	}
	if !connected {
		return ErrChatUnavailable
	}
	_, err = s.DB.Exec(ctx, `INSERT INTO notification_exceptions(user_id,contact_id,allow_dnd_push,allow_call_sound,important) VALUES($1,$2,$3,$4,$5) ON CONFLICT(user_id,contact_id) DO UPDATE SET allow_dnd_push=EXCLUDED.allow_dnd_push,allow_call_sound=EXCLUDED.allow_call_sound,important=EXCLUDED.important,updated_at=now()`, user, contact, v.AllowDNDPush, v.AllowCallSound, v.Important)
	return err
}

type Decision struct{ Push, Visual, MessageSound, CallSound, Mention bool }

func Evaluate(global Preferences, chatMode string, mutedUntil *time.Time, exception *Exception, kind, scope string, now time.Time) Decision {
	mention := kind == "mention"
	call := kind == "call"
	decision := Decision{Push: global.PushEnabled, Visual: true, MessageSound: global.SoundMessages, CallSound: global.SoundCalls, Mention: mention && global.MentionsEnabled}
	if !call && !mention {
		decision.CallSound = false
	}
	if !call {
		decision.MessageSound = false
	}
	if !mention {
		decision.Mention = false
		if kind == "mention" {
			decision.Push = false
			decision.Visual = false
		}
	}
	if mutedUntil != nil {
		if now.Before(*mutedUntil) {
			chatMode = "off"
		} else if chatMode == "off" {
			chatMode = "all"
		}
	}
	switch chatMode {
	case "off":
		decision.Push = false
		decision.Visual = false
		decision.MessageSound = false
		decision.CallSound = false
		decision.Mention = false
	case "silent":
		decision.MessageSound = false
		decision.CallSound = false
	case "mentions":
		if !mention {
			decision.Push = false
			decision.Visual = false
			decision.MessageSound = false
			decision.CallSound = false
			decision.Mention = false
		}
	}
	allowed := exception != nil && (exception.Important && (mention || kind == "important") || call && exception.AllowCallSound)
	dnd := global.DNDUntil != nil && now.Before(*global.DNDUntil) || quietNow(global, now)
	if dnd && !allowed {
		if global.SuppressPush {
			decision.Push = false
		}
		if global.MuteMessageSound {
			decision.MessageSound = false
		}
		if global.MuteCallSound {
			decision.CallSound = false
		}
		if mention && global.SuppressMentions {
			decision.Mention = false
			decision.Visual = false
		}
	}
	if exception != nil && exception.AllowDNDPush && dnd && chatMode != "off" {
		decision.Push = global.PushEnabled
	}
	if scope == "group" && chatMode == "mentions" && !mention {
		decision.Push = false
		decision.Visual = false
	}
	return decision
}
func quietNow(p Preferences, now time.Time) bool {
	if !p.QuietEnabled {
		return false
	}
	loc, e := time.LoadLocation(p.Timezone)
	if e != nil {
		return false
	}
	local := now.In(loc)
	start, e := time.Parse("15:04", p.QuietStart)
	if e != nil {
		return false
	}
	end, e := time.Parse("15:04", p.QuietEnd)
	if e != nil {
		return false
	}
	minute := local.Hour()*60 + local.Minute()
	a := start.Hour()*60 + start.Minute()
	b := end.Hour()*60 + end.Minute()
	if a == b {
		return p.QuietDays != 0
	}
	today := 1 << uint((int(local.Weekday())+6)%7)
	if a < b {
		return p.QuietDays&today != 0 && minute >= a && minute < b
	}
	if minute >= a {
		return p.QuietDays&today != 0
	}
	if minute < b {
		yesterday := 1 << uint((int(local.Weekday())+5)%7)
		return p.QuietDays&yesterday != 0
	}
	return false
}

func (s *Service) LoadDeliveryPolicy(ctx context.Context, user, sender, scope, chat string) (Preferences, ChatPreference, *Exception, error) {
	g := defaults()
	var e Exception
	var ep *Exception
	err := s.DB.QueryRow(ctx, `SELECT sound_messages,push_enabled,sound_calls,mentions_enabled,dnd_until,dnd_mute_message_sound,dnd_suppress_push,dnd_mute_call_sound,dnd_suppress_mentions,quiet_enabled,quiet_days,quiet_start::text,quiet_end::text,timezone FROM notification_preferences WHERE user_id=$1`, user).Scan(&g.SoundMessages, &g.PushEnabled, &g.SoundCalls, &g.MentionsEnabled, &g.DNDUntil, &g.MuteMessageSound, &g.SuppressPush, &g.MuteCallSound, &g.SuppressMentions, &g.QuietEnabled, &g.QuietDays, &g.QuietStart, &g.QuietEnd, &g.Timezone)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return g, ChatPreference{}, nil, err
	}
	c := ChatPreference{Scope: scope, ChatID: chat, Mode: "all"}
	var until *time.Time
	err = s.DB.QueryRow(ctx, `SELECT mode,muted_until FROM chat_notification_preferences WHERE user_id=$1 AND scope=$2 AND chat_id=$3`, user, scope, chat).Scan(&c.Mode, &until)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return g, c, nil, err
	}
	c.MutedUntil = until
	err = s.DB.QueryRow(ctx, `SELECT contact_id::text,allow_dnd_push,allow_call_sound,important FROM notification_exceptions WHERE user_id=$1 AND contact_id=$2`, user, sender).Scan(&e.ContactID, &e.AllowDNDPush, &e.AllowCallSound, &e.Important)
	if err == nil {
		ep = &e
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return g, c, nil, err
	}
	return g, c, ep, nil
}
