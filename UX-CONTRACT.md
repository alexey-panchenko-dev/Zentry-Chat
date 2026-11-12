# Zently 1.1 — актуальный контракт поведения

Этот раздел заменяет устаревшие утверждения ниже там, где они расходятся с текущей реализацией. Исторические заметки оставлены для истории интеграции.

- Авторизация остаётся email + пароль с подтверждением email; телефон не используется как OTP. Пароли до 32 символов принимаются сервером.
- В приложении есть личные заметки, реакции, ответы, закрепления, пересылка, медиа, групповые чаты, вызовы, presence и серверные подтверждения доставки/прочтения. Галочки показывают только реальные серверные события.
- Поиск включает уникальные @username; личная метка контакта видна только владельцу.
- Личные и групповые сообщения имеют клиентский UUID и возможность повторить неудачную отправку. История сохраняет прокрутку, локальные разделители дат и компактный переход к последнему сообщению.
- Правила приватности проверяются сервером; новые сообщения при закрытом доступе идут в запросы. Email входа не раскрывается собеседникам.
- Web Push реализован, но фактическая отправка зависит от VAPID-конфигурации сервера и разрешения браузера. Пока ключи Render не заданы, push недоступен.
- PWA имеет manifest, service worker и офлайн-заглушку; приватные API-ответы не кэшируются.
- Группы поддерживают владельца, администратора и участника. Изменения проверяются сервером.
- Знакомства и лайки до 18 лет выключены серверным флагом по умолчанию; при включении сервер разделяет взрослую и несовершеннолетнюю аудитории.
- Реальное состояние production не следует из локального кода: миграции 031–037 и deployment ещё требуют отдельного согласования.

# Historical integration behavior contract

- Existing API payloads, cookies, query/mutation request functions and call-controller code are preserved.
- Email/password is the actual supported authentication flow. Phone OTP is not invented by the skin.
- User search keeps the existing backend semantics: name fragment or exact email. The backend has no unique username field.
- Saved messages, archive, attachments, message delivery/read acknowledgements and presence are not provided by this API; the skin does not show fake working versions of them.
- One dark message check means saved on the server; there is no unsupported delivery/read inference.
- Native buttons and labels, visible rounded focus, mobile Back, modal call focus and reduced-motion support are maintained.
- Synthesized ringing follows the existing call status. Audio stops on status change/unmount; blocked autoplay has an explicit sound-enable action. Server call expiry remains the friend's original behavior.
- Technical connection information is retained in a collapsible connection section, without changing the original realtime logic.
