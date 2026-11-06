# Contact profiles Implementation Plan

> Agentic workers: use superpowers:subagent-driven-development or executing-plans; preserve existing messenger contracts.

**Goal:** Header avatar/name opens the conversation peer profile with large photo and existing video call action; settings let users add/change/remove a phone and opt into contact-detail visibility.
**Architecture:** Add private profile_contacts table and membership-checked peer profile endpoint. React native dialog shares existing Avatar and CallPanel controller; optional realtime capability flag gates settings on older servers.
**Tech Stack:** Existing Go/pgx/PostgreSQL, React/TypeScript/React Query, native dialog, existing CSS tokens.
**Spec:** User requested contact profile on click, viewing avatar/calling/contact details, and phone editing in settings. Prior preference is autonomous implementation without repeated questions. Defaults: phone/email hidden unless owner opts in; registration email not edited by this feature. No SMS verification or new service.

## Global constraints
- Keep existing pink/lime dark styling and mobile layout; no blue/new dependencies.
- Only conversation members may fetch peer details; never add email/phone to user search, public lists or realtime events.
- Phone optional, normalize formatting spaces/parentheses/hyphens to international +digits, 8..15 digits; preserve absent/empty remove action.
- Old and missing capability flags safely hide settings; header profile can still show known name/photo on older server with clear unavailable details state.
- Use same CallController/start flow and existing busy guard; no duplicate call/signaling implementation.
- Photo enlargement must preserve aspect ratio; dialog supports keyboard/Escape, loading/error/retry and natural scrolling on mobile.

## Interfaces
- GET /api/profile/contact -> {phone:string,show_phone:boolean,show_email:boolean,email:string} for current user only.
- POST same route body {phone,show_phone,show_email} -> same response. Empty phone removes it and forces show_phone=false.
- GET /api/conversations/{id}/profile -> {profile:{id,name,avatar_url?,phone?,email?}}; current user's membership required, peer's privacy applied. 404 outsiders/missing conversation.
- Authenticated GET/realtime/config adds media.contact_profiles=true; preserves all existing fields.
- New migration008_profile_contacts.sql. Profile payload never changes authentication email.
- Client ContactProfile({conversation,onClose,onCall}), ContactSettings, one optional CallPanel requestCall prop {id:string,conversationID:string} passes through existing controller. Header becomes semantic profile button; existing refresh/call controls retained.

## Review Focus
- Outsider/anonymous cannot read contact details; opted-out data omitted, not masked-but-present.
- Valid phone removal persists; invalid edit cannot overwrite previous saved number; email comes only from auth DB.
- Call button closes profile before launching existing modal; no duplicate request on rerender.
- Stale profile fetch cannot display previous peer data after chat switch; settings request cleanup and retry.
- Settings availability handles old-server404 without making text/calls unusable.

## Task1: API
- [x] Test-first phone normalization, privacy/membership/edit/remove routes on real local PostgreSQL.
- [x] Add008 migration, focused store/handlers, register routes and capability; no auth/search field changes.
- [x] Run full Go suite, retain original text/call behavior.

## Task2: UI
- [x] Contact dialog and photo enlargement, loading/error/old-server fallback and keyboard/mobile support.
- [x] Settings phone editing plus explicit show_phone/show_email toggles; only show registered email owner sees from own endpoint.
- [x] Wire header click and profile call action through existing CallPanel controller; preserve original effects and API calls.
- [x] Client tests/build/lint; update DESIGN.md and explicit contract allowlist as authorized.

## Task3: delivery
- [ ] Independent privacy/call-flow review; resolve findings.
- [ ] Commit and push correct repository/master with backward compatibility, verify Vercel and public API readiness.
- [ ] State physical/authenticated browser verification limits honestly; don't call real contacts or publish their data during checks.

Verification: full Go suite passes on real local PostgreSQL;32 client tests, build, lint, original API/signaling contract and strict UI audit pass. Independent reviewer confirmed membership/privacy SQL and existing call path before its final review stopped at an account usage limit; coordinator completed remaining source review and fixed client leading-zero validation and stale private detail display on refresh failure. Authenticated live-browser actions are not claimed verified.
