# Размещение: Neon → Render → Vercel

Подготовленные файлы не публикуют проект автоматически. Размещайте только тестовые данные.
Локальная база Docker не переносится: в Neon будет новая пустая база, миграции API создадут таблицы.

## 1. Репозиторий

Render и Vercel должны получить текущий код из GitHub, включая client и render.yaml.
Перед коммитом проверьте git status. .env и .codex должны оставаться вне Git.
Не загружайте локальную .env на GitHub. Секреты вводятся в настройках хостинга.

## 2. Neon

Создайте проект на бесплатном тарифе, по возможности в европейском регионе рядом с API.
В Connection details скопируйте PostgreSQL connection string с SSL.
Эта строка является секретом; поместите её в DATABASE_URL на Render, а не во frontend.
Для постоянного пула pgx можно начать с прямого подключения Neon.
Сохраните параметры SSL из выданной Neon строки.

## 3. Render

Создайте Blueprint из репозитория: Render прочитает render.yaml.
Проверьте бесплатный тариф Free перед созданием сервиса.
Либо создайте Web Service вручную: Docker, Dockerfile ./Dockerfile, build context корень,
health check /api/ready. PORT назначает Render; не копируйте локальную PORT=9090.
Compose на Render не используется.

Поля окружения:
- DATABASE_URL — строка Neon.
- COOKIE_SECURE=true.
- ALLOWED_ORIGINS — точный HTTPS-адрес production-сайта Vercel, без конечного слеша.
  Если сайт ещё не создан, временно укажите https://deployment-pending.invalid:
  health check продолжит работать, запросы браузера будут закрыты до настройки.
- PUSHER_APP_ID, PUSHER_KEY, PUSHER_SECRET, PUSHER_CLUSTER — из локальных настроек
  того же приложения Pusher. Secret вводите только в секретное поле сервера.
- BREVO_API_KEY — секрет API-ключа Brevo (не отправляйте и не коммитьте его).
- BREVO_FROM_EMAIL — подтверждённый адрес отправителя из Brevo.
- BREVO_FROM_NAME — отображаемое имя отправителя, по умолчанию Zently.
- SITE_URL — https://zentrychat.vercel.app для ссылки в письме.
  Код отправляется через Brevo HTTPS API, что совместимо с бесплатным Render.
  Адрес отправителя должен быть зарегистрирован и подтверждён в Brevo.

Скопируйте выданный HTTPS-адрес API. /api/ready должен вернуть status=ok, database=ok.
Бесплатный Render засыпает после простоя; первый запрос может потребовать ожидания/повтора.
Клиентский timeout 15 секунд может истечь при холодном старте: сначала откройте /api/ready.

## 4. Vercel

Перед публикацией, из client:
npm run configure:hosting -- https://ВАШ-АДРЕС-API.onrender.com

Команда создаёт client/vercel.json с настоящим адресом API.
Закоммитьте получившийся файл в репозиторий перед импортом на Vercel.
Скрипт содержит только публичный адрес, без секретов.
Импортируйте тот же репозиторий, Root Directory = client, Framework = Vite.
Build = npm run build, Output = dist, Node.js = 24.x.
В frontend НЕ нужны DATABASE_URL или PUSHER_SECRET.
Pusher key/cluster клиент получает от API после входа.

После получения production-адреса сайта обновите ALLOWED_ORIGINS на Render.
Оставляйте в списке только нужные адреса. Случайные preview-домены не добавляются автоматически.

Схема: браузер → https://сайт.vercel.app/api/* → Render → Neon.
Внешний rewrite сохраняет обращения клиента к /api и позволяет использовать host-only cookie
через адрес фронтенда. После размещения обязательно проверьте Set-Cookie, login, me и logout
в настоящем браузере. Не включайте кеширование персональных ответов API.

## 5. Проверка

- /api/ready через адрес сайта и напрямую через Render: status=ok, database=ok.
- Два устройства, разные аккаунты, регистрация → вход → поиск → переписка.
- Проверка message.created без обновления страницы.
- Звонок, разрешение камеры/микрофона, принятие, видео/звук и завершение.
- После выхода прежняя сессия не даёт читать API.
- WebRTC использует STUN, TURN пока отсутствует: в некоторых разных сетях видео не соединится.
- Rate limit API считает IP непосредственного прокси: за общим прокси лимит может делиться
  между пользователями. Эта конфигурация рассчитана на небольшую учебную проверку.

Источники:
https://render.com/docs/blueprint-spec
https://render.com/docs/free
https://vercel.com/docs/routing/rewrites
https://neon.com/docs/get-started-with-neon/connect-neon
