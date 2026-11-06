# Conversation controls and birthdays Implementation Plan

> Execute with superpowers:subagent-driven-development or executing-plans. User requests autonomous implementation and publication to the existing repository/master.

**Goal:** Add private history clearing, persistent contact block/unblock with explicit blocked notice, and birth dates instead of manually entered ages, including registration and birthday display.
**Architecture:** Per-member message visibility cursor; directional account block table enforced by server on both text/voice and calls; owner-only stored birth date with computed public age and day/month contact birthday. Existing call/realtime infrastructure retained.
**Tech Stack:** Existing Go/pgx/PostgreSQL, React/TypeScript/React Query, same dark pink/lime UI.
**Spec:** User asked clear/block actions in contact profile and explicit “Вас заблокировали :(” for blocked contact. Clear means only actor's history (implementation default announced). User also requests sidebar toggle above own avatar (already published04692f7), DOB inputДД.ММ.ГГГГ in profile/registration, age computed and birthday shown. Full birth year stays owner-only; contact profile shows month/day.

## Constraints and review focus
- Block is enforced server-side; hiding a composer alone is insufficient. Deny text/voice sends and new calls in either direction while a block exists. Sender sees accurate directional state; owner can unblock.
- Clear uses maximum committed message ID under conversation lock. Never delete peer's messages. Old history/audio/unread/live events cannot reappear for clearing actor. New messages remain visible.
- Membership/auth checked before control state/actions. Block/send/call races use coherent deterministic locking. Existing idempotency maintained, no transport/security weakening.
- Confirm clear and block inside application dialog. No real user history or contacts modified during QA; tests use isolated DB accounts.
- No full birth dates in discovery/search/events or peer profile. Public age computed from date, legacy age fallback retained. Required DOB in new registration UI; backend omission remains compatible with cached clients, provided dates strictly validated.
- Dates1900..today, real calendar dates including leap years, no future dates. JSON ISOYYYY-MM-DD; displayDD.MM.YYYY. No invented age/adult gate or SMS service.
- Capability-gate new actions/settings while older API is deployed. Profile, messages and calls remain usable. Current backend source baseline0d61250 must be preserved except authorized features.

## Task1: Server clear/block
Files: new009_conversation_controls.sql; chat controls store/HTTP/tests; messages/media/read queries; calls store/API block response; router/config.
- [x] Test-first clear isolation/pagination/audio/unread/cursors; block direction/text/voice/new call/accept; outsider/anon; unblock and concurrent send ordering.
- [x] GET /api/conversations/{id}/controls -> {blocked_by_me:boolean,blocked_me:boolean,cleared_through:string}.
- [x] POST same {action:'clear'|'block'|'unblock'} -> same state.
- [x] Capability media.chat_controls=true. Reuse conversation.updated personal event; clear notifies actor only, block/unblock both; call.updated for ended calls if relevant.
- [x] Enforce message/audio history cursors and block errorscontact_blocked; preserve existing Pusher/call signaling.
- [x] Full Go suite on real local DB.

## Task2: Birth-date server
Files: new010_birth_dates.sql; auth register method/handler; discovery profile own get/update/age queries; contact birthday query/tests. Avoid router/controls files owned byTask1.
- [x] Test parsing/calendar/future/leap rules and date-age boundary, registration persistence and legacy compatibility.
- [x] Optional birth_date ISO in registration and profile update; owner GET/api/profile addsbirth_date; public discovery uses computedage but omitsbirth_date.
- [x] Existing auth.Register/UpdateProfile internal signatures remain wrappers for tests/legacy callers; additive methods carry DOB transactionally.
- [x] Member contact profile adds birthday:'MM-DD' only; registration email/auth flows preserved.
- [x] Full Go suite; reconcile shared discovery.go with root's block protection if needed.

## Task3: Client integration
Files: profile controls hook/helpers/tests; ContactProfile/App/VoiceComposer/CallPanel; Discovery/ProfileEditor; Auth and birthday helpers/tests; CSS/DESIGN.
- [x] Confirmation UI clear/block/unblock; controls status query cache shared in profile and messages; invalidate with existing conversation.updated/focus/recovery. Directional messages “Вы заблокировали контакт” / “Вас заблокировали :(”.
- [x] Clear removes local history/cache and filters late events <=cleared_through. Block disables text, voice and profile/header call actions; server still authoritative; unblock restores controls.
- [x] DOB mask/DD.MM.YYYY validation, required registration/profile editor, display computedage and birthday day/month/today appropriately.
- [x] Tests boundary dates/cursors/status direction and call behavior; build/lint/audit. Do not fabricate authenticated browser/device verification.

## Task4: Final integration
- [x] Root reconciles route/source contracts and optional discovery block guard; preserves newest upstream changes.
- [x] Independent review and full tests; report any unavailable review or pre-existing audit findings.
- [x] Publish compatible master; verify Vercel and API new routes/capabilities as accessible, do not claim live blocked user flow without two authenticated sessions.

Additional authorized user steering implemented: chat images/gallery via011, permanently private loginemail via012, hidden delete/removeconversation via013 +ellipsis menu; ownphoto hover/tapupload, profile/settings separation, DOBregistration/profileinputs. See docs/profile-and-conversation-update.md for exactcontracts/verification and limitations.68clienttests/fullGo withrealDB/build/lint/contract/strictaudit pass. Publication/runtimecapabilities to be verified afterpush; no authenticated liveuser flow claimed.
