# Call audio repair plan

Goal: preserve and play both peers' audio, and locate any remaining capture/transport/output failure with actual diagnostics.

1. [x] Reproduce the separate audio/video MediaStream defect with a failing controller test; retain the regression and aggregate all received tracks.
2. [x] Reproduce cancellation while browser permission is pending; allow hang-up and prevent late media permission results from attaching to a cancelled or newer call.
3. [x] Separate audio playback lifecycle from React UI and test late audio tracks, autoplay rejection/retry, track removal, mute/end and cleanup. The UI must not confuse successful play() with receiving a voice signal.
4. [ ] Verify capture/send/receive/output through the local real API using two isolated test sessions. Camera/microphone authorization was explicitly granted by user. Do not use fake media to claim real voice works. Record diagnostics and limitations; stop if user cancels computer control.
5. [ ] Run frontend build/lint and all call/audio tests, preserve signaling API, review diff and publish tested audio fixes to the correct repository. Keep Gmail/account changes isolated from an audio-only production update.

Acceptance: received audio is retained regardless of track/stream arrival order; no camera/mic left attached after cancellation; retry can unlock blocked audio; absent/ended input is shown honestly; local real call shows growing audio counters in both directions. Audible cross-device confirmation and external-network/TURN checks remain distinct from loopback verification.

Environment: Docker was explicitly deferred by user. Use the existing local Go API/PostgreSQL and Vite. No changes to API call/signaling payloads. Browser access must use CUA; never inspect hidden application state or use evaluate to mutate a page.

Verification update (2026-11-06): 12 controller/output tests pass, including both stream arrival orders, streamless late audio, permission cancellation and stale diagnostics. Build, lint and signaling contract check pass. Independent audio review found no P1/P2 issues. Local browser automation is unavailable in this session (native pipe not found); previous local services have stopped. A real two-person audible call is still pending and is not claimed as verified. Audio-only branch is based on master and excludes account/Gmail/backend changes.

