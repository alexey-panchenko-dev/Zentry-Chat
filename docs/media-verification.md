# Media feature verification

## Scope
Voice messages: explicit audio-only recording, maximum 120 seconds/3 MiB, preview/cancel/send/retry, saved server-side and playable by conversation participants. Profile photos: bounded JPEG/PNG input, normalized server-side, preview/upload/replace/remove, versioned shared avatar URLs, account-initial fallback.

## Automated evidence
- Client: `npm --prefix client test`, 24 passing controller/media lifecycle tests; includes delayed microphone permissions, cancel/unmount cleanup, maximum duration, oversized capture, asynchronous final recording chunks and responsive visibility changes.
- Client: `npm --prefix client run build` and `npm --prefix client run lint` passed.
- Contract: `node client/scripts/verify-visual-contract.mjs` passed; original text and signaling API source, transport, deployment and dependencies are preserved. The authorized media backend files are explicitly allowlisted.
- UI static audit: `audit_project.py . --mode strict`, zero findings after explicit form validation ownership and textarea resize rules were added. Static audit does not substitute for browser interaction.
- Go API: full uncached suite passed on real local UTF8 PostgreSQL `zentry_test`; media HTTP tests verify transactional persistence, same-UUID retries/conflicts, recipient access, outsider rejection, byte ranges, photo normalization/replacement/deletion and version propagation. The current full suite also rejects malformed/truncated/header-only audio, absent audio tracks/blocks, invalid Ogg CRC, invalid MP4 box extents, overlong declared durations, and null voice database fields; eight unknown-sized streaming WebM Clusters remain accepted.

## Limits and delivery
- Physical microphone recording/playback and cross-device audible calls have not been verified during this feature task. The available browser tool cannot reach the machine's local development server; it returned connection refused, while local HTTP checks returned 200 for the client and API readiness.
- This branch includes the earlier tested call-audio fix; it does not claim the original live-call problem resolved across external networks. TURN configuration remains a separate issue if peers cannot connect.
- The published site is unchanged until push and API/frontend deployments succeed. GitHub integration previously denied write access to `alexey-panchenko-dev/Zentry-Chat` (403). Render API auto-deploy is disabled in the repository configuration.
- Encoded media fixtures exercise actual supported container structure, not the user's physical recording devices. Server validation is bounded container validation rather than full codec decoding; duration metadata must not be described as a certified playback duration.

Fixture provenance: `api/internal/httpapi/testdata/README.md` records the temporary FFmpeg generator and commands for the four small synthetic encoded fixtures. Go tests require no FFmpeg installation. Parser scope is Opus WebM without lacing, Opus Ogg, and AAC/Opus MP4; codec payloads are not fully decoded.

Live public-site check (2026-11-06): the root browser check of `https://zentrychat.vercel.app/` displayed “API недоступна”; login forms could not be reached. Bounded read-only fetches from this task to the Vercel health/auth routes and Render health endpoint all failed at the network layer, so no HTTP status or cold-start diagnosis is asserted. No deployment settings or credentials were changed.
