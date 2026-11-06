# Profile and conversation update

## User-visible behavior
- Own sidebar avatar opens public-profile editing only: photo, name, bio, city and birth date. The gear opens account/contact/privacy/sound settings and logout. The collapse control remains above the sidebar avatar.
- Birth dates use DD.MM.YYYY in the UI and ISO dates in requests. Registration asks for a real calendar date1900 through today; age is calculated. Only the owner receives the full date. Conversation peers receive birthday month/day, while discovery receives age.
- Login email remains owner-only. The email-sharing switch is removed, responses to peers never include the email, and migration012 disables/prohibits legacy sharing flags.
- Own photo is the file-picker button: plus on hover/focus or touch affordance,15MiBJPEG/PNG preparation and preview/save/cancel. No separate visible file chooser or delete-photo action.
- Chat photo button supports gallery/desktop JPEG/PNG selection, preview/cancel/send/retry and enlargement. Inputs15MiB maximum are prepared to JPEG, longest edge1600px and4MiB maximum upload.
- Contact profile ellipsis contains private chat deletion, contact removal and block/unblock, each with application-owned confirmation. Deleting chat clears only the actor's history and hides the list entry. Removing contact hides it while keeping history. Searching/reopening affects only that actor; new messages restore hidden chats.
- A block is account-pair-wide and enforced on text, voice, images, call creation/acceptance and match-generated messages. Blocked contact sees “Вас заблокировали :(”. Unblocking restores send/call access unless the peer also blocks.

## Server contracts
GET/POST `/api/conversations/{id}/controls`: states `blocked_by_me`, `blocked_me`, `cleared_through` (decimal string), `hidden`; actions `clear`, `delete_chat`, `remove_contact`, `block`, `unblock`.

POST `/api/conversations/{id}/images`: multipart `photo`, `client_message_id`; response follows existing message/realtime contract with `kind:image,image_url`. GET `/api/messages/{messageID}/image` requires membership and an uncleared message.

Registration optionally accepts `birth_date` for compatibility with older clients; the new UI requires it and waits for `/api/health` capability `birth_dates:true`. Own GET/PUT `/api/profile` supports it. Full DOB is absent from public DTOs and realtime payloads.

Authenticated `/api/realtime/config` adds `chat_controls`, `chat_removal`, `chat_images` capability flags. New UI features remain gated when the API is older. Existing signaling transport and client call engine are preserved.

## Verification
68 client tests, production build, lint, source contract and strict UI audit pass. Full Go suite passes on real local PostgreSQL, including concurrency checks for clear/send, block/send, block/accept, block/create, signals and conversation writes; image persistence/permissions/retries and birthday/privacy tests. A shared-database discovery test failed once during overlapping agent suites and passed in the serialized final suite; no test was weakened.

Two independent static reviews found no confirmed P1/P2 issue, including the later hide/delete/removal changes. Actual authenticated browser and cross-device microphone/photo picker flows have not been verified by those automated/static checks. Tests did not mutate real contacts or delete real message histories.

Migrations009–013 apply through normal API startup. Update the backend before expecting capabilities/actions to be available. The UI handles older API capabilities without inventing working controls.
