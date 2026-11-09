package httpapi

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"newsChat/api/internal/auth"
	"newsChat/api/internal/chat"
	"testing"
	"time"
)

func TestGroupAvatarAndSharedPhotoHistoryAreMemberScoped(t *testing.T) {
	f := newControlFixture(t)
	if _, err := f.store.CreateConversation(f.ctx, f.users[0].ID, f.users[2].ID); err != nil {
		t.Fatal(err)
	}
	group, _, err := f.store.CreateGroup(f.ctx, f.users[0].ID, "Gallery", []string{f.users[1].ID, f.users[2].ID})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := f.db.Exec(f.ctx, "DELETE FROM group_chats WHERE id=$1", group.ID); err != nil {
			t.Error(err)
		}
	})
	svc := auth.New(f.db)
	email := "group-media-outsider-" + time.Now().Format("20060102150405.000000000") + "@example.com"
	outsider, err := svc.Register(f.ctx, "Outsider", email, "password-12345")
	if err != nil {
		t.Fatal(err)
	}
	f.users = append(f.users, outsider)
	_, token, _, err := svc.Login(f.ctx, email, "password-12345")
	if err != nil {
		t.Fatal(err)
	}
	f.tokens = append(f.tokens, token)
	photo := image.NewRGBA(image.Rect(0, 0, 12, 10))
	var encoded bytes.Buffer
	if err = jpeg.Encode(&encoded, photo, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.SetGroupPhoto(f.ctx, group.ID, f.users[1].ID, encoded.Bytes(), "image/jpeg"); !errors.Is(err, chat.ErrForbidden) {
		t.Fatalf("non-owner changed group photo: %v", err)
	}
	if _, err = f.store.SetGroupPhoto(f.ctx, group.ID, f.users[0].ID, encoded.Bytes(), "image/jpeg"); err != nil {
		t.Fatal(err)
	}
	for _, member := range f.users[:3] {
		if _, mime, err := f.store.GroupPhoto(f.ctx, group.ID, member.ID); err != nil || mime != "image/jpeg" {
			t.Fatalf("member photo access: %q %v", mime, err)
		}
	}
	if _, _, err = f.store.GroupPhoto(f.ctx, group.ID, f.users[3].ID); !errors.Is(err, chat.ErrNotFound) {
		t.Fatalf("outsider read group avatar: %v", err)
	}
	message, created, _, err := f.store.SendGroupImage(f.ctx, group.ID, f.users[1].ID, "e4f0a161-7855-4bb9-87c1-89cdcb6e3f82", "", encoded.Bytes())
	if err != nil || !created || message.Kind != "image" || message.Body != "" {
		t.Fatalf("send group image: %+v %t %v", message, created, err)
	}
	for _, member := range f.users[:3] {
		images, err := f.store.GroupImages(f.ctx, group.ID, member.ID, 0, 10)
		if err != nil || len(images) != 1 || images[0].ImageURL != message.ImageURL {
			t.Fatalf("member gallery: %+v %v", images, err)
		}
		if _, mime, err := f.store.GroupMessageImage(f.ctx, group.ID, message.ID, member.ID); err != nil || mime != "image/jpeg" {
			t.Fatalf("member image: %q %v", mime, err)
		}
	}
	if _, err = f.store.GroupImages(f.ctx, group.ID, f.users[3].ID, 0, 10); !errors.Is(err, chat.ErrNotFound) {
		t.Fatalf("outsider listed group images: %v", err)
	}
	if _, _, err = f.store.GroupMessageImage(f.ctx, group.ID, message.ID, f.users[3].ID); !errors.Is(err, chat.ErrNotFound) {
		t.Fatalf("outsider read group image: %v", err)
	}
}
