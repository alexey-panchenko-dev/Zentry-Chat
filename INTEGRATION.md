# Zently visual layer for Zentry-Chat

## Media feature branch (2026-11-06)

The original visual-only scope below describes the initial integration. The user subsequently authorized persistent voice messages (up to two minutes) and optional profile photos. The `feat/voice-and-profile-photos` branch extends the original Go API and database for those capabilities, and includes the isolated call-audio repair from `fix/call-audio`.

Voice flow: request microphone on explicit action, stop/preview/cancel/send, preserve failed draft and UUID for retry, display authenticated playback controls in message history. Capture stops on conversation navigation, hidden/mobile chat, call activation, cancellation and unmount. Photos support bounded JPEG/PNG upload, preview, replacement and removal; shared avatars fall back to account initials. Replacement uses versioned URLs.

Media uses the existing database and cookie authentication. Apply migrations through the normal Go API startup, and deploy the API before the new client. Updating the Vercel client alone does not add these endpoints. The current Render configuration disables automatic API deployments; the owner must trigger that deployment.

The implementation is local until a successful push/deployment is explicitly recorded. GitHub integration previously returned 403 for the friend's repository. Automated API tests use a local PostgreSQL database and synthetic accounts/media that clean themselves up. Encoded test media does not verify a physical microphone or cross-device audible calls.

Base: alexey-panchenko-dev/Zentry-Chat at e6f6fe5d32a3820803e7797b9f91364c62c32526.

## Scope

The new client keeps the original backend. Go source, migrations, deployment configuration, API adapter, WebRTC CallController, RemoteAudio and existing tests are unchanged. Presentation changes are in App.tsx, CallPanel.tsx, Visual.tsx and zently.css. The only new runtime module is the original synthesized call-sound layer; it does not modify the call controller or signaling.

Email/password registration remains functional through the same endpoints. Search remains name/exact-email search. Unique usernames, phone OTP, saved messages, archive, attachments, presence and delivered/read receipts need backend support and are deliberately not fabricated in this branch. The existing call expiry is retained.

## Running the actual project

Use the friend's unchanged README, .env.example, compose.yaml and DEPLOY.md. With Docker/PostgreSQL and real Pusher settings, the same services serve the new client. Do not copy the old dependency-free Zently Node server into this project. The original prototype and this integration are separate applications.

For frontend development: cd client, npm ci, npm run dev. The unchanged Vite proxy expects the real Go API at localhost:9090. During design review a different Vite port (5174) and a local-only fixture were used; that fixture is outside this repository and is not a functioning substitute for the backend.

The published API explicitly returns 403 origin_not_allowed for http://127.0.0.1:5174. Its CORS/security settings were not altered or bypassed. For a real browser end-to-end session, run the original local backend stack or deploy the client under an allowed origin; adding a new production origin is an owner configuration step.

## Verification performed

- Original baseline and updated client: npm run build, npm run lint, npm test (four original call-controller tests).
- UI fixture checks: email login form, search, selecting/creating conversation, message submission using Enter, empty-send disabling, desktop and 390px mobile layout. Fixture users/messages are not shipped.
- UI navigation checks additionally cover registration -> chat, login -> chat and logout -> auth without refresh, plus retaining an unsent draft through Settings -> Chats. Presentation-only callbacks refresh the session view after the original cache clear/setQueryData actions; authentication requests are unchanged. Messages stays mounted behind other UI sections so its original pending retry token is preserved.
- node client/scripts/verify-visual-contract.mjs verifies protected files against the base and compares original API calls/payload source and effects with whitespace normalization. It is a contract/source check, not a proof that live media works.
- Real published API: health/ready, anonymous rejection, three synthetic test accounts registered/logged in, user search, conversation creation, message save and retrieval by recipient, idempotent retry, denied outsider access, realtime config, call creation/incoming visibility/cancellation, and logout session revocation. All 13 checks passed. Pusher was enabled and message creation returned realtime=published; this is publisher acceptance, not proof of end-to-end event delivery.
- Media/controller logic: the original four tests cover offer/answer, ICE, device toggles, track release and cancellation while permissions are pending. Real microphone/video across two physical devices has not been established in this environment. TURN is not added.
- No local Go/Docker/PostgreSQL executable was available, so Go's own integration test suite was not executed here.

## Test data cleanup

The local UI fixture holds its data only in memory and is stopped after review; it is not committed. The remote HTTP probe created three clearly named synthetic accounts with prefix zently-ui-qa- and one test conversation/message. The API exposes no delete-account or delete-conversation route. Their removal requires the owner's database access; no existing data was deleted and no schema/cleanup backdoor was introduced.

## Merge

Apply the visual commit/patch onto the specified base or merge this branch. Keep environment files and Pusher credentials private. Rerun client build/lint/tests, then perform the original two-browser live chat and media checks from client/README.md before production deployment. Reconcile future upstream edits to JSX while keeping the API adapter and controller intact.
