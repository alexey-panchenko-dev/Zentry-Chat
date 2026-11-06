# Личные переписки и Pusher

## Как сообщение проходит по проекту

Браузер отправляет POST /api/conversations/{id}/messages с cookie входа.
router.go выбирает обработчик из httpapi/chat.go. signedIn проверяет сессию.
chat.Store проверяет участие в переписке и сохраняет текст в PostgreSQL.
После сохранения realtime/pusher.go отправляет message.created через Pusher.
Браузеры участников слушают private-conversation-<id>.

## API

Все маршруты требуют входа. Для POST нужен application/json.

| Метод | Адрес | Назначение |
|---|---|---|
| GET | /api/users?query=... | До 20 пользователей: часть имени или точная почта; без своей записи |
| POST | /api/conversations | Создать/получить диалог: {"user_id":"UUID собеседника"} |
| GET | /api/conversations | Список своих диалогов, последние созданные сначала |
| POST | /api/conversations/{id}/messages | Отправить {"client_message_id":"UUID","body":"Привет"} |
| GET | /api/conversations/{id}/messages | История: новые сообщения сначала |
| GET | /api/realtime/config | enabled и публичные key/cluster, без secret |
| POST | /api/pusher/auth | Подпись приватного канала для участника диалога |

В списках диалогов и истории: limit от 1 до 100 (по умолчанию 20).
Ответ содержит next_cursor: передайте его как before для следующей страницы.
Пустая строка next_cursor означает конец списка. Сообщения возвращаются от новых
к старым; на экране фронтенд разворачивает порядок каждой страницы.
Числовые идентификаторы сообщений и курсоры — строки, чтобы JavaScript не терял точность.
Возвращаем только публичные ID и имя собеседника, без его почты.
Чужая и несуществующая переписка одинаково возвращают 404.

Текст: 1–1000 символов после удаления крайних пробелов, без NUL.
client_message_id создаётся фронтендом через crypto.randomUUID() один раз.
При сетевом повторе используйте прежние ID и текст. Новый запрос возвращает 201,
повтор — 200 с тем же message.id. Изменение текста/диалога при том же ID — 409.
Для одной пары пользователей всегда существует один диалог, даже при одновременном создании.

## Подключение pusher.com

1. Зарегистрироваться на https://dashboard.pusher.com/ и создать приложение Channels.
2. В App Keys скопировать значения в корневой .env:

```dotenv
PUSHER_APP_ID=
PUSHER_KEY=
PUSHER_SECRET=
PUSHER_CLUSTER=
```

Все четыре значения обязательны для включения Pusher. Без них HTTP-чат работает,
GET /api/realtime/config возвращает enabled=false, авторизация канала — 503.
Частично заполненная конфигурация останавливает запуск с понятной ошибкой.
Secret хранится только на сервере; не передавайте его другу для фронтенда и не коммитьте .env.
Клиенту достаточно публичных key и cluster.

3. Пересоздать API: docker compose up -d --build --wait.
4. Проверить GET /api/realtime/config после входа.
5. Подключить клиент и проверить message.created в Pusher Debug Console.

Настоящая доставка через pusher.com требует вашего приложения и ключей.
Автотесты проверяют подписанный запрос официального SDK через локальный HTTP-сервер,
подпись приватного канала, права участника и сохранение сообщения при ошибке публикации.

## Пример для разработчика React

Установить pusher-js в репозитории фронтенда. Все запросы к нашему API отправлять
с credentials: 'include'. Пример выполняется после login и создания диалога:

```javascript
import Pusher from 'pusher-js'

const api = 'http://localhost:9090'
const configResponse = await fetch(`${api}/api/realtime/config`, { credentials: 'include' })
if (!configResponse.ok) throw new Error('Не удалось получить настройки Pusher')
const config = await configResponse.json()
if (!config.enabled) throw new Error('Заполните настройки Pusher на сервере')

const pusher = new Pusher(config.key, {
  cluster: config.cluster,
  forceTLS: true,
  channelAuthorization: {
    customHandler: async ({ socketId, channelName }, callback) => {
      try {
        const response = await fetch(`${api}/api/pusher/auth`, {
          method: 'POST',
          credentials: 'include',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ socket_id: socketId, channel_name: channelName }),
        })
        if (!response.ok) throw new Error(`Подписка запрещена: ${response.status}`)
        callback(null, await response.json())
      } catch (error) {
        callback(error, null)
      }
    },
  },
})

const channel = pusher.subscribe(`private-conversation-${conversationId}`)
channel.bind('message.created', ({ message }) => {
  // Добавьте сообщение, если message.id ещё нет в состоянии интерфейса.
})

const clientMessageId = crypto.randomUUID()
const response = await fetch(`${api}/api/conversations/${conversationId}/messages`, {
  method: 'POST',
  credentials: 'include',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ client_message_id: clientMessageId, body: 'Привет!' }),
})
if (!response.ok) throw new Error(`Отправка не удалась: ${response.status}`)
const result = await response.json()
// Покажите result.message; событие Pusher может прийти раньше или позже ответа.
// При повторе этого запроса используйте прежний clientMessageId.

// При выходе из аккаунта/размонтировании общего подключения:
// pusher.disconnect()
```

Подпишитесь на канал, дождитесь pusher:subscription_succeeded и затем загрузите
историю. Объединяйте историю и события по message.id, чтобы не потерять сообщения
между запросом истории и подпиской. После восстановления соединения загрузите
историю снова; при необходимости пройдите страницы до уже известного сообщения.
В списке сортируйте ID как BigInt, а не как обычные строки.
Не передавайте JavaScript-токен авторизации: cookie отправляется браузером автоматически.
При выходе отключайте Pusher и очищайте состояние переписки.

## Результат доставки

POST сообщения возвращает realtime:
- published — Pusher принял запрос публикации; получение каждым браузером не гарантируется;
- disabled — ключи ещё не настроены;
- failed — сообщение сохранено, но событие не отправлено.

Повтор с прежним client_message_id снова публикует событие, не дублируя запись в БД.
Автоматической фоновой очереди повторов пока нет: при сбое история остаётся источником истины.
События могут дублироваться или приходить не по порядку — объединяйте их по ID.
После успешного POST /api/conversations сервер публикует `conversation.updated`
в `private-user-<id>` обоих участников с `{"conversation_id":"UUID"}`.
Повторный запрос повторяет уведомление и возвращает тот же диалог. Событие
не содержит данных профиля или сообщения; загрузите список HTTP-запросом.
Личный канал нужно слушать сразу после входа, даже если ещё нет открытого чата.
На каждом `pusher:subscription_succeeded`, в том числе после переподключения,
снова загрузите список: это восполняет события, пропущенные до подписки.
При сбое публикации создание диалога остаётся успешным. Клиент дополнительно
обновляет список раз в 60 секунд в активной вкладке при недоступной подписке,
а также при возвращении фокуса. При рабочем Pusher постоянного опроса нет.

Профиль, знакомства, личные события сообщений/прочтения и счётчики описаны
в [DISCOVERY.md](DISCOVERY.md).
Лимит POST чата — 120 запросов/минуту на IP одного процесса.

## Voice messages and profile photos

Voice messages use `POST /api/conversations/{id}/voice` with multipart `audio`, `duration_ms` (1–120000), and `client_message_id` (UUID). Audio is bounded to 3 MiB and accepts WebM, Ogg, or MP4 MIME types after bounded structural container validation (complete element/page/box bounds, audio tracks, and nonempty sample payloads). `duration_ms` is bounded recorder-supplied metadata, not a server-verified decoded media duration. Validation supports Opus WebM (unlaced blocks, including unknown-length Segment/Cluster), Opus Ogg (page checksums/packet bounds), and AAC/Opus MP4 (normal sample tables or fragmented runs). Other WebM lacing and Ogg codecs are unsupported. Known container durations/timestamps above 120 seconds are rejected. This does not fully decode codec payloads or certify the actual playback duration or audible quality. The API responds with `{message,realtime}` and publishes the existing `message.created` event. Voice message fields are `kind:"voice"`, `audio_url`, and `duration_ms`; text fields remain compatible. A repeated UUID must match conversation, bytes, normalized MIME, and duration exactly or returns 409. Media and message live in one atomic database row, so retries cannot orphan blobs.

`GET /api/messages/{messageID}/audio` requires conversation membership and supports byte ranges. Unknown and inaccessible messages return 404. Media responses use private/no-store caching and nosniff.

Own photo upload uses multipart `photo` at `POST /api/profile/photo`; `DELETE` removes it. Both return `{avatar_url}` (empty after delete). Inputs are bounded to 2 MiB, decoded only as JPEG/PNG after dimensions are checked (4096 per side, 16 million pixels), and normalized to PNG with a 2 MiB output bound. `GET /api/profiles/{userID}/photo` requires a signed-in account. Account and public-user responses include optional `avatar_url`. Photo replacement is atomic and changes a revision UUID in the returned `avatar_url` query string (`?v=...`); auth/search/conversation responses use the latest revision. Reads do not cache stale images.

## Delivery receipts
GET /api/delivery returns at most 100 pending incoming message envelopes for the signed-in recipient, oldest first. It changes no receipts. After receiving the response the browser POSTs /api/delivery with message_ids (1–100 positive int64 strings). Membership and cleared cursors are checked. Delivery never changes unread or viewed cursors. GET conversation read state adds peer_last_delivered_id. Read receipts also advance delivery, but only POST /read advances viewed_through. Local history clearing advances unread state without fabricating a viewed receipt. Migration 014 is required; old clients keep the existing read API.
