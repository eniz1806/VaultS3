import { apiFetch } from './client'

export interface BackupRecord {
  id: string
  type: string
  target: string
  startTime: string
  endTime: string
  objects: number
  size: number
  status: string
}

export interface BackupStatus {
  enabled: boolean
  running: boolean
  targets: number
}

export function listBackups(): Promise<BackupRecord[]> {
  return apiFetch<BackupRecord[]>('/backups')
}

export function getBackupStatus(): Promise<BackupStatus> {
  return apiFetch<BackupStatus>('/backups/status')
}

export function triggerBackup(): Promise<void> {
  return apiFetch<void>('/backups/trigger', { method: 'POST' })
}
