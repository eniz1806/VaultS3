import { apiFetch } from './client'

export interface Bucket {
  name: string
  createdAt: string
  size: number
  objectCount: number
  maxSizeBytes?: number
  maxObjects?: number
  policy?: Record<string, unknown>
}

export function listBuckets(): Promise<Bucket[]> {
  return apiFetch<Bucket[]>('/buckets')
}

export function createBucket(name: string): Promise<Bucket> {
  return apiFetch<Bucket>('/buckets', {
    method: 'POST',
    body: JSON.stringify({ name }),
  })
}

export function getBucket(name: string): Promise<Bucket> {
  return apiFetch<Bucket>(`/buckets/${name}`)
}

export function deleteBucket(name: string): Promise<void> {
  return apiFetch<void>(`/buckets/${name}`, { method: 'DELETE' })
}

export function setBucketPolicy(name: string, policy: string): Promise<void> {
  return apiFetch<void>(`/buckets/${name}/policy`, {
    method: 'PUT',
    body: policy,
  })
}

export function setBucketQuota(name: string, maxSizeBytes: number, maxObjects: number): Promise<void> {
  return apiFetch<void>(`/buckets/${name}/quota`, {
    method: 'PUT',
    body: JSON.stringify({ maxSizeBytes, maxObjects }),
  })
}
