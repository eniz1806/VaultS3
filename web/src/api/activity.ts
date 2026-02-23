import { apiFetch } from './client'

export interface ActivityEntry {
  time: string
  method: string
  bucket: string
  key: string
  status: number
  size: number
  clientIP: string
}

export function getActivity(limit = 50): Promise<ActivityEntry[]> {
  return apiFetch<ActivityEntry[]>(`/activity?limit=${limit}`)
}
