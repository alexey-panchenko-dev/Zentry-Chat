# Zently Messaging Fixes Implementation Plan

> **Execution:** Native implementation in this session. The user asked to follow this plan.

**Goal:** Make registration land in the app, keep text entry continuous, expose chat deletion, support photo captions, improve photo framing and mobile typing, and make voice recording stable on phones.

**Architecture:** Keep the existing React composer and Go chat API. Use the existing message `body` for an optional image caption, update its database constraint with a migration, and keep image/audio bytes and authorization unchanged. Fix focus and narrow-screen layout in the current composer components and styles.

**Tech Stack:** React 19, TypeScript, Vite, Go, PostgreSQL migrations, Node test runner.

**Spec:** User requests in this conversation and attached screenshots; image-caption choice confirmed as “Подпись, которая отправляется вместе с фото”.

## Global Constraints

- Preserve existing chat, image, audio, and read-receipt behavior.
- A group with two selected contacts means three participants including the creator; this server/UI change is already in `master` and awaiting confirmation of the Render deployment.
- Photo captions use the existing 1,000 character message limit; captions are optional.
- Photo proportions remain unchanged; only the surrounding padding and touch hover treatment change.
- Do not change the repository owner or deploy to another repository.

## Review Focus

- Registration must leave the signup screen only after register and login both succeed; failures remain actionable.
- The composer stays focused after send, but does not steal focus while a profile, call, or image viewer is open.
- Photo caption and image upload remain one authorized message; empty captions and retries must work.
- Touch hover/focus must not leave a lime outline around a photo; desktop hover remains discernible.
- Voice stop must retain the final audio chunk; permission errors and empty recordings must not break composer layout.

---

### Task 1: Registration handoff and message composer focus

**Files:** `client/src/App.tsx`, `client/src/Groups.tsx`, `client/src/zently.css`

- [x] After successful register-plus-login, reload the app at `#chats`; leave the auth form visible on either request failure.
- [x] Keep the textarea focusable during send with `readOnly`, then refocus after the mutation has settled and the input is enabled.
- [x] Focus the direct or group composer when its chat becomes visible; do not focus hidden chats.
- [x] Set mobile composer text to at least 16px to prevent iOS focus zoom.
- [x] Verify with `npm run build` and the existing client tests.

### Task 2: Show chat deletion in the contact menu

**Files:** `api/internal/httpapi/chat.go`, `client/src/useConversationControls.ts`, `client/src/ContactProfile.tsx`

- [x] Advertise `chat_removal: true` in realtime capabilities, matching the existing `delete_chat` endpoint and menu gate.
- [ ] Verify the profile “…” menu exposes “Удалить чат” and that its existing confirmation/hide behavior remains intact.
- [ ] Verify through capability and controls tests; deployment needs the API service to pick up the pushed server change.

### Task 3: Send photo captions

**Files:** `client/src/ImageComposer.tsx`, `client/src/App.tsx`, `api/internal/httpapi/images.go`, `api/internal/chat/images.go`, new migration `api/internal/database/migrations/020_image_captions.sql`, image integration tests.

- [x] Add an optional caption input to the selected-photo preview and submit it with the image upload.
- [x] Accept captions up to 1,000 characters; normalize and validate them on the server.
- [x] Allow a non-empty `body` for `kind='image'` in the database while retaining image MIME/size checks.
- [x] Render the caption beneath the image and preserve idempotent retries.
- [ ] Verify empty caption, non-empty caption, overlong caption rejection, and retry behavior.

### Task 4: Photo bubble sizing and touch behavior

**Files:** `client/src/zently.css`, `client/src/ImageMessage.tsx` only if required.

- [x] Reduce the visible side padding around image bubbles to 2px while preserving bottom metadata spacing.
- [x] Restrict lime hover outline to hover-capable devices; use a small pink focus treatment for keyboard users.
- [ ] Verify portrait and landscape photos on touch and desktop; keep original aspect ratios.

### Task 5: Voice recorder reliability and mobile layout

**Files:** `client/src/voice-recorder.ts`, `client/src/VoiceComposer.tsx`, `client/src/zently.css`, `client/tests/voice-recorder.test.mjs`.

- [x] Reproduce the empty recording with a recorder event sequence and add a regression test for delayed/final `dataavailable` delivery.
- [x] Finalize only after the recorder has delivered its final chunk; keep the microphone stream alive until then.
- [x] Keep mic/send/text controls in one row on small screens; display recording errors in a non-overlapping status area.
- [ ] Verify successful preview/send, short/empty recording feedback, permission denial, and 2-minute stop.

### Task 6: Verify publication and service state

**Files:** no code changes unless verification reveals a defect.

- [x] Run the client build and test suite.
- [ ] Run Go tests and API integration checks; Go is not installed in this environment, so use repository CI.
- [ ] Confirm Vercel checks for the resulting `master` commit.
- [ ] Confirm the Render API has deployed the same commit; the dashboard previously required email verification, so report that dependency clearly if it remains inaccessible.
- [ ] Confirm group creation with two contacts plus the creator after the API is current.
