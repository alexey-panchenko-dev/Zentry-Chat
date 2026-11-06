package httpapi

import (
	"errors"
	"testing"
)

type conversationPublisher struct {
	testPublisher
	channels  []string
	failFirst bool
}

func (p *conversationPublisher) Publish(channel, event string, data any) error {
	p.channels = append(p.channels, channel)
	if event != "conversation.updated" {
		return errors.New("unexpected event")
	}
	if p.failFirst && len(p.channels) == 1 {
		return errors.New("temporary failure")
	}
	return p.testPublisher.Publish(channel, event, data)
}

func TestConversationNotification(t *testing.T) {
	for _, fail := range []bool{false, true} {
		pub := &conversationPublisher{failFirst: fail}
		h := &chatHandlers{publisher: pub}
		h.notifyConversation("dialog", "sender", "recipient")
		if len(pub.channels) != 2 || pub.channels[0] != "private-user-sender" || pub.channels[1] != "private-user-recipient" {
			t.Fatalf("both users must be notified even if the first publication fails: %v", pub.channels)
		}
		payload, ok := pub.data.(map[string]string)
		if !ok || len(payload) != 1 || payload["conversation_id"] != "dialog" {
			t.Fatalf("notification should only contain the dialog ID: %v", pub.data)
		}
	}
}
