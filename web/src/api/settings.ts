import { apiFetch } from './client'

export interface Settings {
  server: {
    address: string
    port: number
    domain?: string
    shutdownTimeoutSecs: number
    tlsEnabled: boolean
  }
  storage: {
    dataDir: string
    metadataDir: string
  }
  features: {
    encryption: boolean
    compression: boolean
    accessLog: boolean
    rateLimit: boolean
    replication: boolean
    scanner: boolean
    tiering: boolean
    backup: boolean
    oidc: boolean
    lambda: boolean
    debug: boolean
  }
  lifecycle: {
    scanIntervalSecs: number
    auditRetentionDays: number
  }
  rateLimit?: {
    requestsPerSec: number
    burstSize: number
    perKeyRps: number
    perKeyBurst: number
  }
  memory: {
    maxSearchEntries: number
    goMemLimitMb?: number
  }
}

export function getSettings(): Promise<Settings> {
  return apiFetch<Settings>('/settings')
}
