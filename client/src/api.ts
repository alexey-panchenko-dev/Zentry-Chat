import axios from 'axios'

export const api = axios.create({
  baseURL: '/api',
  withCredentials: true,
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' },
})

export type User = { id: string; name: string; avatar_url?: string }
export type Conversation = { id: string; participant: User; unread_count?: number }
export type Profile = User & { age: number | null; bio: string; city: string }
export type ReadState = { unread_count: number; last_message_id: string; peer_last_read_id: string }
export type Message = {
  id: string
  conversation_id: string
  sender_id: string
  kind?: 'text' | 'voice' | 'system'
  audio_url?: string
  duration_ms?: number
  body: string
  created_at: string
}
export type MessagePage = { messages: Message[]; next_cursor: string }

export function errorText(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const code = error.response?.data?.error
    const labels: Record<string, string> = {
      profile_required: 'Заполните возраст и описание в настройках профиля.',
      invalid_profile: 'Проверьте профиль: имя обязательно, описание и город — до 100 символов, возраст — целое число лет.',
      invalid_media: 'Файл не подходит. Проверьте формат и размер и повторите.',
      invalid_voice: 'Не удалось принять запись. Запишите сообщение заново (до 2 минут и 3 МБ).',
      invalid_photo: 'Не удалось сохранить фото. Выберите другой JPG или PNG.',
      invalid_credentials: 'Неверная почта или пароль.',
      email_already_registered: 'Эта почта уже зарегистрирована.',
      unauthorized: 'Сессия закончилась. Войдите снова.',
      rate_limit_exceeded: 'Слишком много запросов. Подождите минуту.',
    }
    return labels[code] ?? error.response?.data?.message ?? code ?? 'API недоступна. Проверьте Docker и порт 9090.'
  }
  return error instanceof Error ? error.message : 'Неизвестная ошибка'
}
