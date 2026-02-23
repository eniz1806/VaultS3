import { apiFetch } from './client'

export interface AccessKey {
  accessKey: string
  maskedSecret: string
  createdAt: string
  isAdmin: boolean
}

export interface CreatedKey {
  accessKey: string
  secretKey: string
  createdAt: string
}

export function listKeys(): Promise<AccessKey[]> {
  return apiFetch<AccessKey[]>('/keys')
}

export function createKey(): Promise<CreatedKey> {
  return apiFetch<CreatedKey>('/keys', { method: 'POST' })
}

export function deleteKey(accessKey: string): Promise<void> {
  return apiFetch<void>(`/keys/${accessKey}`, { method: 'DELETE' })
}
