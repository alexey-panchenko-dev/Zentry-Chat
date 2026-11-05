# Account tools and call audio update

This user-requested update extends the original visual-only integration. It changes the backend only for password recovery and Saved Messages, and fixes the client audio stream association. Existing auth/chat/call API contracts are preserved.

Login now locks the account while verifying its password and issuing the session, coordinating with reset so an in-flight old-password login cannot survive session revocation. Mail requests are processed asynchronously with bounded capacity and identical public responses even at saturation; under overload a request may be dropped and the operator receives a generic capacity log. Clients should retry if mail does not arrive. This best-effort in-process transport is not a durable mail queue.

## Deployment

Deploy both the Go API on Render and the client on Vercel. The existing render.yaml disables automatic deployment (`autoDeployTrigger: off`). Migration 004_account_tools.sql runs through the existing startup migration runner. A frontend-only deployment cannot activate the new routes.

Set these private Render environment values: PUBLIC_APP_URL=https://zentrychat.vercel.app, GMAIL_SENDER, GMAIL_CLIENT_ID, GMAIL_CLIENT_SECRET, GMAIL_REFRESH_TOKEN. Never add credential values to GitHub or chat. Enable Gmail API in the owner's Google Cloud project and authorize the intended sender with offline access and only https://www.googleapis.com/auth/gmail.send. Use a Google OAuth client, not a normal Gmail password. Gmail API uses HTTPS because free Render blocks SMTP ports. Testing-mode OAuth grants may expire quickly; configure production consent appropriately before relying on recovery. Owner configuration and live Gmail delivery remain pending.

Routes: POST /api/auth/forgot-password, POST /api/auth/reset-password, POST /api/conversations/saved. Recovery requests use identical responses for existing/missing accounts, a 60-second account cooldown and existing IP limits. Tokens are 32 random bytes, only SHA-256 hashes are stored, expire after 15 minutes, and can be redeemed exactly once even concurrently. Reset replaces the bcrypt password and revokes all sessions atomically. Original 12–72 UTF-8 byte password policy remains. Tokens travel in the frontend fragment, are removed from the address bar on load, and are never returned in API responses or logs. Missing configuration returns recovery_unavailable. A mail transport failure invalidates the undelivered token and logs only a generic message; the HTTP response never discloses account existence.

Saved Messages reuses membership-protected conversations/messages and server persistence. Dedicated self-conversation creation accepts no user ID. The unique pair makes creation idempotent. Other users cannot read/write notes, and calls to self are rejected. No localStorage fake chat is used.

## Call audio

The old ontrack handler could replace an audio stream with a separate video stream. The client now retains all received tracks if browser stream associations differ. A regression test demonstrates the prior loss and verifies the fix. This confirmed defect does not establish the only cause of silence in the owner's live call. Audio diagnostics show sent/received audio bytes and microphone level. Native player chrome was replaced with a compact retry panel; the original dedicated audio output lifecycle remains.

## Verification

Go 1.27.3 tests run against isolated local PostgreSQL 18.6 with a UTF-8 test database. Coverage includes reset expiry, cooldown, token hashing, concurrent one-time redemption, old password/session rejection, new password login, Saved Messages idempotency/persistence/outsider denial and self-call denial. All original Go tests also run with TEST_DATABASE_URL. React build/lint and five controller tests pass, including separate audio/video stream regression. Browser checks cover recovery validation/unconfigured response, local-only login, Saved Messages send/reload, desktop/mobile and bottom menu toggle. No real account passwords were changed during tests. Live Gmail delivery and live two-device audio after deployment still require verification.

Docker flow verification is pending: this Windows machine originally lacked Docker and WSL. Official WSL 3.0.3 was installed and VirtualMachinePlatform enabled; Windows reports RebootPending. The Docker Desktop installer was downloaded from Docker's official site, its Authenticode signature verified, and installation resumed after enabling the Windows feature. Do not report container startup as verified until Docker Engine runs after reboot and `docker compose up --build -d` plus user flows complete. Keep the existing standalone prototype off port 3000 before starting Compose; stop the temporary local API on 9090 as well. Do not delete production data or Docker volumes.
