import { apiFetch } from './client'

export interface NotificationConfig {
  bucket: string
  webhookURL: string
  events: string[]
}

export function listNotifications(): Promise<NotificationConfig[]> {
  return apiFetch<NotificationConfig[]>('/notifications')
}
