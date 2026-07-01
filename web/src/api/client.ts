import axios, { type AxiosInstance } from 'axios'
import type {
  Account,
  ApiItem,
  ApiList,
  ApiToken,
  Delivery,
  Rule,
  Sink,
  Source,
  SyncedPeer,
  Template,
  TokenCreated,
} from '@/types'

const TOKEN_KEY = 'tmf_api_token'

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? ''
}

export function setToken(token: string): void {
  if (token) {
    localStorage.setItem(TOKEN_KEY, token)
  } else {
    localStorage.removeItem(TOKEN_KEY)
  }
}

const http: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  timeout: 20000,
})

http.interceptors.request.use((config) => {
  const token = getToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// --- Accounts ---
export const accountsApi = {
  list: () => http.get<ApiList<Account>>('/accounts').then((r) => r.data.data),
  get: (id: number) => http.get<ApiItem<Account>>(`/accounts/${id}`).then((r) => r.data.data),
  create: (body: Record<string, unknown>) =>
    http.post<ApiItem<Account>>('/accounts', body).then((r) => r.data.data),
  update: (id: number, body: Record<string, unknown>) =>
    http.put<ApiItem<Account>>(`/accounts/${id}`, body).then((r) => r.data.data),
  remove: (id: number) => http.delete(`/accounts/${id}`),
}

// --- Sinks ---
export const sinksApi = {
  list: () => http.get<ApiList<Sink>>('/sinks').then((r) => r.data.data),
  types: () => http.get<ApiList<string>>('/sinks/types').then((r) => r.data.data),
  get: (id: number) => http.get<ApiItem<Sink>>(`/sinks/${id}`).then((r) => r.data.data),
  create: (body: Record<string, unknown>) =>
    http.post<ApiItem<Sink>>('/sinks', body).then((r) => r.data.data),
  update: (id: number, body: Record<string, unknown>) =>
    http.put<ApiItem<Sink>>(`/sinks/${id}`, body).then((r) => r.data.data),
  remove: (id: number) => http.delete(`/sinks/${id}`),
}

// --- Sources ---
export const sourcesApi = {
  list: () => http.get<ApiList<Source>>('/sources').then((r) => r.data.data),
  get: (id: number) => http.get<ApiItem<Source>>(`/sources/${id}`).then((r) => r.data.data),
  create: (body: Record<string, unknown>) =>
    http.post<ApiItem<Source>>('/sources', body).then((r) => r.data.data),
  update: (id: number, body: Record<string, unknown>) =>
    http.put<ApiItem<Source>>(`/sources/${id}`, body).then((r) => r.data.data),
  remove: (id: number) => http.delete(`/sources/${id}`),
  sync: (accountId: number) =>
    http
      .post<ApiList<SyncedPeer>>(`/sources/sync?account_id=${accountId}`)
      .then((r) => r.data.data),
  start: (id: number) => http.post(`/sources/${id}/start`),
  stop: (id: number) => http.post(`/sources/${id}/stop`),
}

// --- Templates ---
export const templatesApi = {
  list: () => http.get<ApiList<Template>>('/templates').then((r) => r.data.data),
  create: (body: Record<string, unknown>) =>
    http.post<ApiItem<Template>>('/templates', body).then((r) => r.data.data),
  update: (id: number, body: Record<string, unknown>) =>
    http.put<ApiItem<Template>>(`/templates/${id}`, body).then((r) => r.data.data),
  remove: (id: number) => http.delete(`/templates/${id}`),
}

// --- Rules ---
export const rulesApi = {
  list: () => http.get<ApiList<Rule>>('/rules').then((r) => r.data.data),
  get: (id: number) => http.get<ApiItem<Rule>>(`/rules/${id}`).then((r) => r.data.data),
  create: (body: Record<string, unknown>) =>
    http.post<ApiItem<Rule>>('/rules', body).then((r) => r.data.data),
  update: (id: number, body: Record<string, unknown>) =>
    http.put<ApiItem<Rule>>(`/rules/${id}`, body).then((r) => r.data.data),
  remove: (id: number) => http.delete(`/rules/${id}`),
}

// --- Deliveries ---
export const deliveriesApi = {
  list: (status = '', limit = 50, offset = 0) =>
    http
      .get<ApiList<Delivery>>('/deliveries', { params: { status, limit, offset } })
      .then((r) => r.data.data),
  retry: (id: number) => http.post(`/deliveries/${id}/retry`),
}

// --- Tokens ---
export const tokensApi = {
  list: () => http.get<ApiList<ApiToken>>('/tokens').then((r) => r.data.data),
  create: (name: string) =>
    http.post<ApiItem<TokenCreated>>('/tokens', { name }).then((r) => r.data.data),
  revoke: (id: number) => http.delete(`/tokens/${id}`),
}
