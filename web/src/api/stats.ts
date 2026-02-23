import { apiFetch } from './client'

export interface BucketStat {
  name: string
  size: number
  objectCount: number
  maxSizeBytes?: number
  maxObjects?: number
}

export interface Stats {
  totalBuckets: number
  totalObjects: number
  totalSize: number
  uptimeSeconds: number
  goroutines: number
  memoryMB: number
  buckets: BucketStat[]
}

export function getStats(): Promise<Stats> {
  return apiFetch<Stats>('/stats')
}
