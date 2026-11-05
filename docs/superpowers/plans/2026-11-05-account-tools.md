# Password recovery and Saved Messages

Goal: recover an account through its registered Gmail/email address and provide private server-backed notes.

Approved direction: user requested recovery, selected Gmail delivery, and requested a personal saved chat. Extend the existing Go/PostgreSQL/React architecture, keeping the messenger style and existing chat/call APIs.

- [x] Add reset token storage, one-use expiry (15 minutes), per-account cooldown (60 seconds), atomic password replacement and revocation of all sessions. Send through Gmail API over HTTPS; never expose tokens in responses or logs. Link uses a configured HTTPS frontend URL and fragment token.
- [x] Add recovery UI to login/settings with validation, expired-link feedback and no false email-delivery success when unconfigured. Google OAuth credentials must be supplied privately by owner before live sending.
- [x] Add dedicated self-conversation endpoint and migration allowing equal user IDs. Reuse membership-protected message storage, pin Saved Messages, hide self calling. Ensure outsiders cannot access notes.
- [ ] Run Go unit/integration tests where test DB is available, frontend build/lint/original call tests and browser checks. Document unavailable integration checks and deployment requirements honestly.

Security decisions: identical recovery response for existing/missing accounts; only hash reset tokens in database; account lock serializes token creation/reset; no change to password until valid token submitted. Gmail credentials remain environment-only. No sends or password changes for the owner's real account during testing.
