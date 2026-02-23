import { apiFetch } from './client'

export interface LoginResponse {
  token: string
}

export interface MeResponse {
  user: string
  accessKey: string
}

export function login(accessKey: string, secretKey: string): Promise<LoginResponse> {
  return apiFetch<LoginResponse>('/auth/login', {
    method: 'POST',
    body: JSON.stringify({ accessKey, secretKey }),
  })
}

export function getMe(): Promise<MeResponse> {
  return apiFetch<MeResponse>('/auth/me')
}
