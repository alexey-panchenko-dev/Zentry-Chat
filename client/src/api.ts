import axios from 'axios'

export const api = axios.create({
  baseURL: '/api',
  withCredentials: true,
  timeout: 15000,
  headers: { 'Content-Type': 'application/json' },
})

export type User = { id: string; name: string; username?: string; avatar_url?: string }
export type Conversation = { id: string; participant: User; display_name?:string; unread_count?: number }
export type Profile = User & { age: number | null; birth_date?:string; bio: string; city: string }
export type ReadState = { unread_count: number; last_message_id: string; peer_last_read_id: string; peer_last_delivered_id?:string }
export type Message = {
  id: string
  conversation_id: string
  sender_id: string
  kind?: 'text' | 'voice' | 'system' | 'image'
  image_url?: string
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
      contact_blocked: 'Сообщения и звонки недоступны: контакт заблокирован.',
      invalid_birth_date: 'Проверьте дату рождения: настоящая дата от 1900 года до сегодня.',
      profile_required: 'Заполните дату рождения и описание в настройках профиля.',
      invalid_profile: 'Проверьте профиль: имя обязательно, описание и город — до 100 символов, дата рождения — от 1900 года до сегодня.',
      invalid_image: 'Не удалось сохранить фото. Выберите другой JPG или PNG.',
      invalid_media: 'Файл не подходит. Проверьте формат и размер и повторите.',
      invalid_voice: 'Не удалось принять запись. Запишите сообщение заново (до 2 минут и 3 МБ).',
      invalid_photo: 'Не удалось сохранить фото. Выберите другой JPG или PNG.',
      invalid_credentials: 'Неверная почта или пароль.',
      email_already_registered: 'Эта почта уже зарегистрирована.',
      username_taken: 'Этот @юзер уже занят. Выберите другой.',
      invalid_username: 'От 3 до 40 символов: латиница, цифры и _; первый символ — буква.',
      invalid_alias: 'Имя контакта должно быть не длиннее 80 символов.',
      unauthorized: 'Сессия закончилась. Войдите снова.',
      rate_limit_exceeded: 'Слишком много запросов. Подождите минуту.',
    }
    return labels[code] ?? error.response?.data?.message ?? code ?? 'API недоступна. Проверьте Docker и порт 9090.'
  }
  return error instanceof Error ? error.message : 'Неизвестная ошибка'
}
