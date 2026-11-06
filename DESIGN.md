# Zently visual integration

## Intent
Apply the user's approved Zently messenger visual identity to the existing React client. The existing Go/PostgreSQL/Pusher backend and WebRTC controller remain authoritative; media additions are authorized by docs/superpowers/plans/2026-11-06-voice-and-photos.md.

## Visual language
Dark three-panel messenger, Manrope, #0B090E background, #141116 surfaces, #211C23 cards, #FF1493 pink and #B7FF39 lime. Pink outgoing bubbles, subtle pink edge light on incoming bubbles, lime navigation and send controls. Glass is limited to composer and call controls. No blue. Initial avatars use an account username when supplied, otherwise the existing account name; contact-label aliases are not introduced.

## Tokens and ownership
Runtime visual tokens live in client/src/zently.css. Reusable Brand/Avatar are in client/src/Visual.tsx. Existing calls.ts, RemoteAudio.tsx and main.tsx remain unchanged. api.ts adds optional voice/photo fields only. CallPanel.tsx changes layout, modal focus and optional synthesized audio only; it delegates start/accept/reject/end/device actions to the original CallController.

## Supported behavior
Preserve email/password auth, original user search, pagination, message retry/idempotency, Pusher subscriptions, call polling and WebRTC media handling. This integration does not add unsupported backend capabilities or pretend that a Pusher publication is a read receipt. Mobile separates the conversation list and active chat, with a Back action. Call sound is a presentation-only preference.

## Verification
Original build, lint and four call-controller tests pass. See INTEGRATION.md for API checks, UI fixture coverage, protected-file verification and remaining gaps. Test fixtures stay outside the repository and are not shipped.

## Authorized media behavior
VoiceRecorder owns audio-only capture and bounded local drafts (120 seconds, 3 MiB). VoiceComposer owns preview/cancel/send/retry with the same UUID and preserved text. Navigation, calls, page hide and unmount release capture; late permission results stop tracks. VoiceMessage uses native audio controls for honest playback and seeking, without invented waveform data. CallPanel reports active/busy state without changing its controller.
ProfilePhoto owns JPG/PNG selection (2 MiB), preview, replacement/removal and pending/error feedback. The server validates and re-encodes photos. Avatar remains shared owner of account initials and authenticated API photos with image-error fallback. External image URLs are excluded. Existing session/query caches refresh after photo changes. Controls retain dark Manrope/pink/lime tokens and Russian labels. Native audio controls are intentionally platform-owned, keyboard accessible and responsive.

Auth and search forms own localized inline validation with first-invalid-field focus; native constraint checks are retained without browser popups. Composer textarea uses the shared resize-none rule to preserve action visibility.

The single composer surface contains the idle microphone beside the text draft and send button. Recording/requesting/preview replace the text controls within that surface; the mounted text input retains its draft and returns after send/cancel. Voice preview has only its own send action.

MediaAvailability requires explicit authenticated server capability flags from /realtime/config before mounting voice/photo controls. Missing flags, old servers and lookup errors keep uploads/recording unavailable with a compact explanation; checks repeat every 30 seconds until supported. Text and call flows retain their existing behavior.

Profile photos accept JPEG/PNG inputs up to 15 MiB. The browser prepares a proportional JPEG copy with longest edge at most 1024px, without upscaling, before upload. The stored upload stays below the existing 2 MiB server/database limit. Pending preparation disables upload controls; stale preparation results and temporary object URLs are released on replacement/unmount.
