# Zently Chat, Groups, Photos, and Controls Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the requested chat/profile/group updates in Zently’s existing pink/lime style while preserving the current account, messaging, call, and photo flows.

**Architecture:** Extend the existing React client and authenticated Go API: share photo history and group media through membership-checked routes, keep deletion visibility in per-member history watermarks, and use accessible form controls for date and phone entry.

**Tech Stack:** React, TypeScript, CSS, Vite, Go, PostgreSQL.

**Spec:** Current user request and attached screenshot in this conversation.

## Global Constraints

- Keep the existing chat and photo-viewer behavior intact.
- Keep the requested glass styling limited to the photo close button and received text bubbles.
- Keep sent bubbles fully opaque; preserve readable contrast and existing bubble direction/corners.
- Keep fallback colors for browsers without backdrop-filter.

## Review Focus

- Close button remains visible and legible over light and dark photos.
- Message text stays readable while the chat background subtly shows through.
- Sent and received messages retain distinct pink/graphite identity.
- Photo messages retain source proportions and consistent corners.
- Narrow mobile layouts retain a reachable close button.
- Group avatar stays square with its icon/photo centered inside; do not allow header flex to stretch it.
- Only group members may see group photos; only the owner may change the group avatar.
- Phone country choices must show country and calling code and allow manual country prefix selection.
- Date of birth must be entered as day/month/year using the mouse wheel, touch drag, or direct typing.

---

### Task 1: Add frosted glass to the image close control

**Files:**
- Modify: `client/src/zently.css` (existing `.z-chat-image-dialog .z-close-image` rule)

- [x] Use a layered translucent surface, backdrop blur/saturation, subtle border and inset highlight; add a clear hover/focus state without changing its fixed placement.
- [x] Keep a legible opaque fallback for browsers without backdrop-filter.
- [x] Inspect the open photo viewer at desktop and mobile widths. (CSS build and interaction semantics checked; live visual check follows publish.)

### Task 2: Add a restrained glass surface to message bubbles

**Files:**
- Modify: `client/src/zently.css` (final effective `.z-bubble` own/received rules)

- [x] Add backdrop blur and subtle surface highlights to received text bubbles while preserving the graphite/pink palette, neon edge, and asymmetric corners.
- [x] Keep sent bubbles fully opaque; exclude system bubbles and image-only bubbles from the translucent treatment.
- [x] Preserve text contrast and layering against the chat background.

### Task 3: Keep voice recording controls visible

**Files:**
- Modify: `client/src/zently.css` (active `.z-voice-composer` layout)

- [x] Give voice recording, cancel, and send controls reserved grid columns so the buttons cannot cover each other on narrow screens.
- [x] Keep the voice send button solid lime and touch-sized.

### Task 4: Persist unsent drafts per chat

**Files:**
- Create: `client/src/message-drafts.ts`
- Modify: `client/src/App.tsx` (`Messages` draft state and persistence)
- Test: `client/tests/message-drafts.test.mjs`

- [x] Store and restore each draft by account and conversation, capped at the existing 1000-character message limit.
- [x] Remove the stored draft when the input is cleared after a successful send; tolerate unavailable browser storage.
- [x] Add tests for restoration/isolation, clearing, storage failure, and length cap.

### Task 5: Show shared photos in contact profiles

**Files:**
- Modify: `api/internal/chat/images.go`, `api/internal/httpapi/chat.go`, `api/internal/httpapi/router.go`
- Modify: `client/src/ContactProfile.tsx`, `client/src/zently.css`
- Test: `api/internal/httpapi/images_integration_test.go`

- [x] Add a paginated, membership-checked photo-history route that respects each member’s clear-history marker.
- [x] Show received and sent photos in the profile with thumbnail preview, full-size opening, empty/error/loading states, and a load-more action.
- [x] Add integration coverage for unrelated accounts and clear-history visibility.

### Task 6: Align the call action with the chat header

**Files:**
- Modify: `client/src/zently.css` (`.z-call-panel.is-idle` placement)

- [x] Move the desktop call action into the header’s reserved action area and retain its mobile placement.

### Task 7: Make group identity and media usable

**Files:** `client/src/Groups.tsx`, `client/src/App.tsx`, `client/src/zently.css`, `api/internal/chat/groups.go`, new group media files and migrations.

- [x] Fix group icon geometry in both list and header: square frame with a centered participants icon or group photo.
- [x] Open a group profile from the group header; show member count, message count, and group photo gallery.
- [x] Let the group owner set a square group avatar; restrict photo retrieval and image history to group members.
- [x] Add captioned group photo messages and paginated group photo history.
- [ ] Add database integration coverage for avatar/image permissions and verify migrations on a running PostgreSQL instance.

### Task 8: Add phone country selection and readable formatting

**Files:** `client/src/PhoneNumberField.tsx`, `client/src/phone-number.ts`, `client/src/ContactSettings.tsx`, tests.

- [x] Show country names and calling codes, format national digits with readable spacing, and recognize a manually typed international prefix.
- [x] Test Kazakhstan, Russia, Ukraine, Czechia, and general international prefixes.

### Task 9: Add a rolling, manually editable birth date

**Files:** `client/src/BirthDateWheel.tsx`, `client/src/birth-wheel.ts`, `client/src/App.tsx`, `client/src/Discovery.tsx`, `client/src/zently.css`.

- [x] Split day, month, and year into separate wheel columns; support mouse wheel, touch drag, arrow keys, and direct numeric entry.
- [x] Automatically move focus from day to month to year while typing.
- [x] Keep existing real-date validation and use the same control in registration and profile settings.
- [x] Cover date field parsing, stepping, and manual numeric bounds with tests.
- [x] Make day/month drums wrap at their ends, animate wheel and touch steps, and zero-pad a one-digit manual entry when the user leaves its field.

### Task 10: Make chat deletion choices clear

**Files:** `api/internal/chat/controls.go`, `api/internal/httpapi/controls.go`, `client/src/ContactProfile.tsx`, `client/src/conversation-controls.ts`, `client/src/useConversationControls.ts`.

- [x] Replace the redundant “Удалить контакт” menu item with “Удалить у всех” and “Удалить только у меня”.
- [x] Clear both participants’ current history markers for global deletion and keep the chat available for future messages.
- [x] Keep local deletion scoped to the current user and add server-side integration coverage.

### Task 11: Diagnose the laptop microphone path

- [ ] Inspect microphone permission/device selection, sender track state, and outbound WebRTC audio stats without attributing the issue to the backend prematurely.
- [ ] Use the in-call diagnostics from a laptop and phone test to identify whether the failure is capture, WebRTC transport, or remote playback.
- [ ] Fix only the failing stage and verify with a two-device call.

### Task 12: Verify and publish

**Files:**
- No additional source files.

- [ ] Run the client production build, lint, all client tests, and applicable Go tests/migrations.
- [ ] Review the changed diff and verify the published site after the authorized push.
- [ ] Record results and update this plan’s checkboxes.
