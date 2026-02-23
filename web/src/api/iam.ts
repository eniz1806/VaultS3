import { apiFetch } from './client'

export interface IAMUser {
  name: string
  policyArns: string[]
  groups: string[]
  createdAt: string
}

export interface IAMGroup {
  name: string
  policyArns: string[]
  createdAt: string
}

export interface IAMPolicy {
  name: string
  document: string
  createdAt: string
}

export function listUsers(): Promise<IAMUser[]> {
  return apiFetch<IAMUser[]>('/iam/users')
}

export function createUser(name: string): Promise<IAMUser> {
  return apiFetch<IAMUser>('/iam/users', { method: 'POST', body: JSON.stringify({ name }) })
}

export function deleteUser(name: string): Promise<void> {
  return apiFetch<void>(`/iam/users/${name}`, { method: 'DELETE' })
}

export function attachUserPolicy(userName: string, policyName: string): Promise<void> {
  return apiFetch<void>(`/iam/users/${userName}/policies`, { method: 'POST', body: JSON.stringify({ policyName }) })
}

export function detachUserPolicy(userName: string, policyName: string): Promise<void> {
  return apiFetch<void>(`/iam/users/${userName}/policies/${policyName}`, { method: 'DELETE' })
}

export function addUserToGroup(userName: string, groupName: string): Promise<void> {
  return apiFetch<void>(`/iam/users/${userName}/groups`, { method: 'POST', body: JSON.stringify({ groupName }) })
}

export function removeUserFromGroup(userName: string, groupName: string): Promise<void> {
  return apiFetch<void>(`/iam/users/${userName}/groups/${groupName}`, { method: 'DELETE' })
}

export function listGroups(): Promise<IAMGroup[]> {
  return apiFetch<IAMGroup[]>('/iam/groups')
}

export function createGroup(name: string): Promise<IAMGroup> {
  return apiFetch<IAMGroup>('/iam/groups', { method: 'POST', body: JSON.stringify({ name }) })
}

export function deleteGroup(name: string): Promise<void> {
  return apiFetch<void>(`/iam/groups/${name}`, { method: 'DELETE' })
}

export function attachGroupPolicy(groupName: string, policyName: string): Promise<void> {
  return apiFetch<void>(`/iam/groups/${groupName}/policies`, { method: 'POST', body: JSON.stringify({ policyName }) })
}

export function detachGroupPolicy(groupName: string, policyName: string): Promise<void> {
  return apiFetch<void>(`/iam/groups/${groupName}/policies/${policyName}`, { method: 'DELETE' })
}

export function listPolicies(): Promise<IAMPolicy[]> {
  return apiFetch<IAMPolicy[]>('/iam/policies')
}

export function createPolicy(name: string, document: string): Promise<IAMPolicy> {
  return apiFetch<IAMPolicy>('/iam/policies', { method: 'POST', body: JSON.stringify({ name, document }) })
}

export function deletePolicy(name: string): Promise<void> {
  return apiFetch<void>(`/iam/policies/${name}`, { method: 'DELETE' })
}
