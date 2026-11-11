# Zently 5.0 — release checks

Date: 2026-11-11. Base: 12486e0. Production branch: master.

## Performance
- Early session request runs before React and chat code load.
- Login, chat, Pusher and full emoji catalogue are separate chunks.
- Initial message page: 20. Older messages remain available via pagination.
- Hidden group queries stop. Healthy realtime suppresses fallback polling.
- Message images use small previews; full images load on opening. Audio/video preload is disabled.
- 3,935 local Apple emoji images are requested on demand. Catalogue loads only after opening the picker; initial grid has 84 buttons.
- Browser-ready timing is available on .z-shell[data-chats-ready-ms]; it includes navigation until conversation-list readiness.

## Security fixes
- Fixed login failures being reset before the next attempt; three failures now enforce the requested ten-minute account lock. Regression covers expiry and independent accounts.
- Go build toolchain and Docker builder updated from 1.27.3 to 1.27.4 to fix GO-2026-6859 (net/http server crash).
- Pending registration credentials cannot be overwritten by a second signup while a verification code exists. Integration regression covers this.
- No user row or session is created before successful email verification.
- Request limits cover GET, PUT and DELETE as well as POST. Spoofed forwarding headers are ignored.
- Verified-account budgets: 300 reads and 120 writes per minute, independent of other accounts.
- Concurrent API work is bounded to 64 requests; uploads, original media downloads and image-preview decoding share a two-job memory budget. Media reads queue briefly; upload overload returns Retry-After. MIME-type case cannot bypass the gate.
- Database/request contexts have a fifteen-second deadline. Direct API responses also receive MIME-sniffing and framing protection.
- Birth dates cannot be omitted to bypass the minimum registration age of fourteen.
- Headers are capped at 32 KiB; existing header/body/read/write timeouts remain.
- Frontend CSP, anti-framing, MIME sniffing, referrer and permission headers added.
- All production npm dependencies checked: zero reported advisories.
- govulncheck on Go 1.27.4: zero reachable/package vulnerabilities. Module-only advisory GO-2026-5932 concerns unused openpgp; the application imports bcrypt, not openpgp. Latest stable x/crypto is v0.58.0.

## Validation
- 91 frontend tests passed, TypeScript and production build passed.
- Go integration suite uses an isolated disposable local PostgreSQL instance, including auth/OTP, private chat/media access, groups, favorites and calls. No production data modified.
- OTP regressions verify cooldown, expiry, five failed attempts, single use, and invalidation of all old sessions after recovery.
- In-process overload regressions verify 64 active requests, two active media jobs, queued previews and spoofed proxy headers. No production flood performed.
- Browser smoke checks: personal/group emoji messages, focus after sending, picker search and actual Apple images, full-size photos and group profile.
- Call signaling/media lifecycle tested automatically; a live two-person microphone call has not been repeated in this release.

## Deployment dependencies
Frontend is deployed through Vercel from master. Both root-directory layouts carry the same security headers and rewrites. The hosting setup script preserves those headers.

After push, the direct Render API and the Vercel proxy both returned version 5.0.0 with new API security headers: backend updating occurred despite the empty accessible Render workspace. Direct service settings remain inaccessible; actual auto-deploy behavior differs from the blueprint's disabled trigger. Health exposes the public Render commit revision when available to verify subsequent releases precisely. Real email delivery still requires verified BREVO_API_KEY/BREVO_FROM_EMAIL configuration and a recipient-side test. No new paid service was created.

This is a bounded code and regression audit, not a volumetric DDoS exercise or a guarantee against every vulnerability. Infrastructure protection of the direct Render origin still requires its owner's account. Vercel supplies platform DDoS mitigation: https://vercel.com/docs/vercel-firewall/ddos-mitigation . Review methodology: https://cheatsheetseries.owasp.org/cheatsheets/Denial_of_Service_Cheat_Sheet.html .
