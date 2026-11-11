# Авторизация

POST /api/auth/register принимает name, email, password, birth_date и возвращает 202.
Созданный аккаунт нельзя использовать до подтверждения почты.
POST /api/auth/verify-email принимает email и шестизначный code. После успешного
подтверждения клиент автоматически входит в аккаунт.
POST /api/auth/resend-verification повторно отправляет код; между письмами минимум 60 секунд.
POST /api/auth/login принимает email, password и возвращает 200 с {"user":...} и cookie.
POST /api/auth/password-reset/request принимает email. Ответ одинаковый для существующих
и неизвестных аккаунтов, чтобы не раскрывать, зарегистрирован ли адрес.
POST /api/auth/password-reset/confirm принимает email, code и новый password; все старые
сессии отзываются после смены пароля.
GET /api/auth/me возвращает 200 с {"user":...} или 401, если сессии нет или она истекла.
POST /api/auth/logout удаляет текущую сессию и cookie, возвращает 204 без тела.
На всех POST обязателен Content-Type: application/json, для logout передайте {}.

Имя при регистрации является уникальным @юзером: латинские буквы, цифры и `_`. Почта приводится к нижнему регистру.
Дата рождения обязательна; сервер проверяет минимальный возраст 14 лет, календарную корректность и отсутствие будущей даты.
Новый пароль при регистрации и сбросе: 6–16 символов; обязательны латинские строчные и заглавные буквы, цифра и один из символов `& * $ %`. Для входа принимаются ранее установленные пароли, в том числе не соответствующие новому правилу.
Пароль не обрезается и не изменяется. Ошибки имеют вид {"error":"code"}.
Неверные данные — 400, неверные реквизиты входа — 401, занятая почта — 409,
неподтверждённая почта — 403, неподдерживаемый Content-Type — 415,
слишком большой JSON — 413, лимиты запросов/кодов и временная блокировка входа — 429.
После трёх неверных паролей вход для почты блокируется на 10 минут.

## Проверка PowerShell

Сначала настройте Brevo API в корневом .env, затем из корня запустите Docker Compose.
После регистрации возьмите шестизначный код из письма и вставьте его в строку `$code`.

```powershell
$base = "http://localhost:9090"
$email = "test-$([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds())@example.com"
$registration = @{name="Test"; email=$email; password='Zently7$'; birth_date="2000-01-01"} | ConvertTo-Json
Invoke-RestMethod "$base/api/auth/register" -Method Post -ContentType "application/json" -Body $registration
$code = Read-Host "Код из письма"
@{email=$email; code=$code} | ConvertTo-Json | Invoke-RestMethod "$base/api/auth/verify-email" -Method Post -ContentType "application/json"
$login = @{email=$email; password='Zently7$'} | ConvertTo-Json
Invoke-RestMethod "$base/api/auth/login" -Method Post -ContentType "application/json" -Body $login -SessionVariable chatSession
Invoke-RestMethod "$base/api/auth/me" -WebSession $chatSession
Invoke-RestMethod "$base/api/auth/logout" -Method Post -ContentType "application/json" -Body '{}' -WebSession $chatSession
Invoke-RestMethod "$base/api/auth/me" -WebSession $chatSession
```

Последний запрос должен вернуть 401. WebSession сохраняет cookie между запросами.

## Для разработчика фронтенда

```javascript
await fetch('http://localhost:9090/api/auth/login', {
  method: 'POST',
  credentials: 'include',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email, password }),
})
```

credentials: 'include' нужен и для me/logout. Токен в localStorage не хранить:
браузер сам передаёт HttpOnly-cookie. Ответы не содержат хеша пароля или токена.
Локально используйте localhost для фронтенда и API, не смешивайте с 127.0.0.1.
ALLOWED_ORIGINS — разрешённые адреса фронтенда через запятую без конечного слеша.
После изменения .env пересоздайте контейнеры через docker compose up -d.
Для публикации под HTTPS задайте COOKIE_SECURE=true. Текущая SameSite=Lax cookie
предполагает размещение клиента и API на одном сайте; разные сайты требуют отдельной настройки.

## Подтверждение почты и восстановление

Коды случайные шестизначные, живут 10 минут, принимают максимум 5 попыток и
повторно отправляются не чаще одного раза в минуту. В базе лежат bcrypt-хеши,
а не сами коды. Все ответы авторизации имеют Cache-Control: no-store и ограничение
частоты запросов по IP.

Почта отправляется через HTTPS endpoint Brevo `POST https://api.brevo.com/v3/smtp/email`.
Задайте `BREVO_API_KEY`, `BREVO_FROM_EMAIL`, `BREVO_FROM_NAME` и `SITE_URL`.
Адрес отправителя должен быть подтверждён в Brevo. API-ключ задавайте только
в локальном `.env` или в секретах Render, никогда в исходном коде.

Регистрация хранится во временной таблице `pending_registrations`; строка в `users`
создаётся только после верного кода. У заявки одна уникальная почта и один уникальный
@юзер. Код истекает через 10 минут; вместе с ним истекает незавершённая заявка.
Сброс пароля тоже требует отдельного запроса кода, а новый пароль сохраняется только
после его проверки.

## Реализация и ограничения

- bcrypt хеширует пароли с DefaultCost; SHA-256 применяется только к случайным токенам.
- Сессии действуют 7 дней; срок проверяется в PostgreSQL на каждом запросе me.
- Выход отзывает одну сессию. Истёкшие записи пока автоматически не очищаются.
- Ограничитель: 20 POST-запросов в минуту на IP, в памяти одного процесса;
  при перезапуске сбрасывается. За reverse proxy нужна отдельная настройка доверенных IP.
- Проверяются Origin, JSON Content-Type; запрещены неизвестные JSON-поля и несколько JSON-объектов.
- Для будущих маршрутов чата нужно также проверять сессию и участие в переписке.
- Подтверждение почты обязательно для новых аккаунтов; ранее созданные аккаунты
  отмечаются подтверждёнными миграцией, чтобы сохранить доступ.

## Автотесты

Docker-сборка запускает модульные тесты. Локально из api:

```powershell
go mod tidy
go fmt ./...
go vet ./...
go test ./...
```

Интеграционный TestSessionLifecycle запускается только при заданном TEST_DATABASE_URL.
Используйте отдельную тестовую базу: тест применяет миграции, создаёт пользователя,
проверяет повторную почту, неверный пароль, вход, выход и истечение сессии,
затем удаляет своего пользователя. Без переменной этот тест пропускается.
