import { apiFetch } from './client'

export interface LambdaTrigger {
  id: string
  functionURL: string
  events: string[]
  keyFilter: string
}

export interface LambdaStatus {
  enabled: boolean
  totalTriggers: number
  buckets: number
  queueDepth: number
}

export interface BucketTriggers {
  bucket: string
  triggers: LambdaTrigger[]
}

export function getLambdaStatus(): Promise<LambdaStatus> {
  return apiFetch<LambdaStatus>('/lambda/status')
}

export function listLambdaTriggers(): Promise<BucketTriggers[]> {
  return apiFetch<BucketTriggers[]>('/lambda/triggers')
}

export function setBucketTriggers(bucket: string, triggers: LambdaTrigger[]): Promise<void> {
  return apiFetch<void>(`/lambda/triggers/${bucket}`, { method: 'PUT', body: JSON.stringify({ triggers }) })
}

export function deleteBucketTriggers(bucket: string): Promise<void> {
  return apiFetch<void>(`/lambda/triggers/${bucket}`, { method: 'DELETE' })
}
