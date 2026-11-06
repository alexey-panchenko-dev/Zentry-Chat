# Voice messages and profile photos Implementation Plan

> For agentic workers: use superpowers:subagent-driven-development or superpowers:executing-plans. Follow tasks in order of interface dependencies.

**Goal:** Record, preview, cancel, send and play persistent private voice messages; upload, replace and remove profile photos.

**Architecture:** Extend the existing Go/PostgreSQL API with authenticated binary media endpoints and transactional voice message creation. React MediaRecorder owns microphone capture; shared Avatar owns photo display and initials fallback. PostgreSQL stores bounded media; no external storage service is required.

**Tech Stack:** Existing Go/pgx, React/TypeScript, browser MediaRecorder and audio element.

**Spec:** User approved voice records up to 120 seconds stored in the existing database (2026-11-06), and requested optional profile photos. Photo upload/replace/remove and initials fallback are implementation defaults.

## Global Constraints
- Existing pink/lime dark theme, Russian labels, glass only in composer/control surfaces.
- Voice recording <=120 seconds and <=3 MiB. Supported browser formats: WebM, Ogg, MP4; unsupported browsers get an honest notice.
- Photo <=2 MiB input; decoded JPEG/PNG only, dimension limit and server re-encoding. Authenticated display; no arbitrary SVG or external URLs.
- Extend rather than replace existing text routes/signaling. Voice and photos must persist and enforce account/conversation permissions.
- No audio starts recording without a user action. Stop microphone on cancel, navigation, error and unmount. Preserve failed upload draft for retry with same UUID.
- Binary endpoints use current cookie auth; voice reads require conversation membership. Photos readable to signed-in users, writable only by owner.
- Do not claim live microphone/browser verification when unavailable. Publication currently blocked by GitHub integration 403.

## Review Focus
- Late microphone permission after cancel/unmount stops tracks and never resumes recording.
- Oversize/invalid media rejected before DB mutation; inaccessible voice IDs return 404.
- Duplicate upload UUID cannot create duplicate messages or overwrite a different payload.
- Retry keeps local draft; successful send releases local object URL.
- Photo decoding bounded before full decode, avatar image errors fall back to first two account letters.

## Task 1: Persistent media API
Files: new migration 005_media.sql (004 is reserved by separate account-tools branch), new media store/handlers/tests; existing chat/auth models and queries; router.
- [x] Write and run failing validation/access/idempotency tests.
- [x] POST /api/conversations/{id}/voice multipart fields audio, duration_ms, client_message_id. Response {message,realtime}; voice Message adds kind:'voice', audio_url and duration_ms. Text remains backward compatible.
- [x] GET /api/messages/{messageID}/audio with membership check, correct MIME, private caching and byte ranges for seeking.
- [x] POST /api/profile/photo multipart photo; DELETE same path. Response {avatar_url:string}; GET /api/profiles/{userID}/photo authenticated. User/PublicUser adds optional avatar_url.
- [x] Keep publication events message.created and original idempotent text flow.
- [x] Test valid/invalid media, limits, own photo replacement/deletion, outsider voice read, repeated/conflicting voice UUID. Run complete Go suite.

## Task 2: Recorder, player and photo UI
Files: new voice recorder controller/tests, VoiceComposer.tsx, VoiceMessage.tsx, ProfilePhoto.tsx; existing App.tsx, api.ts, Visual.tsx, zently.css, DESIGN.md.
- [x] Recorder tests: stop/preview/cancel, permission rejection, late permission cleanup, automatic 120s stop, disposal.
- [x] Integrate voice composer and message player with Task1 routes; text composition preserved. Use multipart header override, consistent errors, pending states and retry.
- [x] Integrate photo preview/upload/remove in settings; clear input so reselect works; refresh account/conversations/search caches after changes.
- [x] Shared Avatar renders owned photo URL and falls back to account letters; accessible keyboard controls and narrow layout.
- [x] Run client tests/build/lint, inspect contract changes and update only explicitly authorized protection rules.

## Task 3: Review and delivery
- [x] Independent review media permissions, resource cleanup, limits, idempotency and mobile UI.
- [x] Record actual automated results and browser/live limits. Update plan and DESIGN.md.
- [x] Commit on feature branch; attempt correct-repository publish only with available credentials. Never claim deployment on rejection.

## Actual verification and delivery limits
- Implementation and independent review completed; review fixes include versioned avatars, structural media validation, non-null voice database checks, and streaming WebM Cluster boundaries.
- Real local PostgreSQL full uncached Go suite passed; client 24 tests, production build, lint, visual contract check, and strict UI audit passed.
- Four synthetic FFmpeg-encoded fixtures cover Opus WebM/Ogg and normal/fragmented AAC MP4. They do not verify physical microphone capture or audible cross-device playback.
- Local commit and transfer artifacts are prepared on the feature branch. Publishing to the correct GitHub repository remains blocked by a confirmed 403; no deployment is claimed.
