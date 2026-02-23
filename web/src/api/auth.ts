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

export interface OIDCConfigResponse {
  enabled: boolean
  issuerUrl?: string
  clientId?: string
}

export interface OIDCLoginResponse {
  token: string
  user: string
  email: string
}

export function getOIDCConfig(): Promise<OIDCConfigResponse> {
  return apiFetch<OIDCConfigResponse>('/auth/oidc/config')
}

export function oidcLogin(idToken: string): Promise<OIDCLoginResponse> {
  return apiFetch<OIDCLoginResponse>('/auth/oidc', {
    method: 'POST',
    body: JSON.stringify({ idToken }),
  })
}
