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

// Versioning
export function getBucketVersioning(name: string): Promise<{ versioning: string }> {
  return apiFetch<{ versioning: string }>(`/buckets/${name}/versioning`)
}

export function setBucketVersioning(name: string, versioning: string): Promise<void> {
  return apiFetch<void>(`/buckets/${name}/versioning`, {
    method: 'PUT',
    body: JSON.stringify({ versioning }),
  })
}

// Lifecycle
export interface LifecycleRule {
  expirationDays: number
  prefix: string
  status: string
}

export function getLifecycleRule(name: string): Promise<{ rule: LifecycleRule | null }> {
  return apiFetch<{ rule: LifecycleRule | null }>(`/buckets/${name}/lifecycle`)
}

export function setLifecycleRule(name: string, rule: LifecycleRule): Promise<void> {
  return apiFetch<void>(`/buckets/${name}/lifecycle`, {
    method: 'PUT',
    body: JSON.stringify(rule),
  })
}

export function deleteLifecycleRule(name: string): Promise<void> {
  return apiFetch<void>(`/buckets/${name}/lifecycle`, { method: 'DELETE' })
}

// CORS
export interface CORSRule {
  allowed_origins: string[]
  allowed_methods: string[]
  allowed_headers?: string[]
  max_age_secs?: number
}

export function getCORSConfig(name: string): Promise<{ rules: CORSRule[] }> {
  return apiFetch<{ rules: CORSRule[] }>(`/buckets/${name}/cors`)
}

export function setCORSConfig(name: string, rules: CORSRule[]): Promise<void> {
  return apiFetch<void>(`/buckets/${name}/cors`, {
    method: 'PUT',
    body: JSON.stringify({ rules }),
  })
}

export function deleteCORSConfig(name: string): Promise<void> {
  return apiFetch<void>(`/buckets/${name}/cors`, { method: 'DELETE' })
}
