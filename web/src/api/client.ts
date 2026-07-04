import axios, { type AxiosInstance } from 'axios'
import type {
  Account,
  ApiItem,
  ApiList,
  ApiPage,
  ApiToken,
  BootstrapStatus,
  Delivery,
  LoginFlow,
  Me,
  Rule,
  RuleMeta,
  RulePreviewResult,
  SharedProxy,
  Sink,
  SinkDescriptor,
  SinkTestResult,
  Source,
  SyncedPeer,
  Template,
  TemplatePreview,
  TelegramApp,
  TokenCreated,
  MediaSettings,
  MediaSettingsRequest,
  MediaS3TestResult,
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
  timeout: 60000,
})

http.interceptors.request.use((config) => {
  const token = getToken()
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 会话失效（401）时清理本地凭证并跳转登录页；auth 端点自身的 401 由调用方处理。
http.interceptors.response.use(
  (resp) => resp,
  (error) => {
    const url: string = error?.config?.url ?? ''
    const status: number | undefined = error?.response?.status
    if (status === 401 && !url.startsWith('/auth/')) {
      setToken('')
      void import('@/router').then(({ router }) => {
        if (router.currentRoute.value.name !== 'login') {
          router.push({ name: 'login' })
        }
      })
    }
    return Promise.reject(error)
  },
)

// --- Auth ---
export const authApi = {
  bootstrapStatus: () =>
    http.get<ApiItem<BootstrapStatus>>('/auth/bootstrap').then((r) => r.data.data),
  bootstrapAdmin: (username: string, password: string) =>
    http.post<ApiItem<{ token: string }>>('/auth/bootstrap', { username, password }).then((r) => r.data.data),
  login: (username: string, password: string) =>
    http
      .post<ApiItem<{ authenticated: boolean; token: string }>>('/auth/login', { username, password })
      .then((r) => r.data.data),
  me: () => http.get<ApiItem<Me>>('/auth/me').then((r) => r.data.data),
  logout: () => http.post('/auth/logout'),
}

// --- Accounts ---
export const accountsApi = {
  list: () => http.get<ApiList<Account>>('/accounts').then((r) => r.data.data),
  get: (id: number) => http.get<ApiItem<Account>>(`/accounts/${id}`).then((r) => r.data.data),
  create: (body: Record<string, unknown>) =>
    http.post<ApiItem<Account>>('/accounts', body).then((r) => r.data.data),
  update: (id: number, body: Record<string, unknown>) =>
    http.put<ApiItem<Account>>(`/accounts/${id}`, body).then((r) => r.data.data),
  remove: (id: number) => http.delete(`/accounts/${id}`),
  // Telegram 登录 flow（验证码登录）。
  login: {
    start: (id: number) =>
      http.post<ApiItem<LoginFlow>>(`/accounts/${id}/login/start`).then((r) => r.data.data),
    code: (id: number, flowId: string, code: string) =>
      http
        .post<ApiItem<LoginFlow>>(`/accounts/${id}/login/code`, { flow_id: flowId, code })
        .then((r) => r.data.data),
    password: (id: number, flowId: string, password: string) =>
      http
        .post<ApiItem<LoginFlow>>(`/accounts/${id}/login/password`, { flow_id: flowId, password })
        .then((r) => r.data.data),
    status: (id: number) =>
      http.get<ApiItem<LoginFlow | null>>(`/accounts/${id}/login/status`).then((r) => r.data.data),
    cancel: (id: number, flowId: string) =>
      http.post(`/accounts/${id}/login/cancel`, { flow_id: flowId }),
    // 扫码登录。
    qr: {
      start: (id: number) =>
        http.post<ApiItem<LoginFlow>>(`/accounts/${id}/login/qr/start`).then((r) => r.data.data),
      status: (id: number, flowId: string) =>
        http
          .get<ApiItem<LoginFlow | null>>(`/accounts/${id}/login/qr/status`, {
            params: { flow_id: flowId },
          })
          .then((r) => r.data.data),
      refresh: (id: number, flowId: string) =>
        http
          .post<ApiItem<LoginFlow>>(`/accounts/${id}/login/qr/refresh`, { flow_id: flowId })
          .then((r) => r.data.data),
      cancel: (id: number, flowId: string) =>
        http.post(`/accounts/${id}/login/qr/cancel`, { flow_id: flowId }),
    },
  },
}

// --- Sinks ---
export const sinksApi = {
  list: () => http.get<ApiList<Sink>>('/sinks').then((r) => r.data.data),
  types: () => http.get<ApiList<string>>('/sinks/types').then((r) => r.data.data),
  meta: () => http.get<ApiList<SinkDescriptor>>('/sinks/meta').then((r) => r.data.data),
  get: (id: number) => http.get<ApiItem<Sink>>(`/sinks/${id}`).then((r) => r.data.data),
  create: (body: Record<string, unknown>) =>
    http.post<ApiItem<Sink>>('/sinks', body).then((r) => r.data.data),
  update: (id: number, body: Record<string, unknown>) =>
    http.put<ApiItem<Sink>>(`/sinks/${id}`, body).then((r) => r.data.data),
  remove: (id: number) => http.delete(`/sinks/${id}`),
  test: (body: Record<string, unknown>) =>
    http.post<ApiItem<SinkTestResult>>('/sinks/test', body, { timeout: 120000 }).then((r) => r.data.data),
  testExisting: (id: number, body: Record<string, unknown>) =>
    http.post<ApiItem<SinkTestResult>>(`/sinks/${id}/test`, body, { timeout: 120000 }).then((r) => r.data.data),
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
      .post<ApiList<SyncedPeer>>(`/sources/sync?account_id=${accountId}`, undefined, { timeout: 120000 })
      .then((r) => r.data.data),
  syncStream: async (
    accountId: number,
    onPeers: (peers: SyncedPeer[]) => void,
    signal?: AbortSignal,
    force = false,
  ) => {
    const headers = new Headers()
    const token = getToken()
    if (token) {
      headers.set('Authorization', `Bearer ${token}`)
    }
    const resp = await fetch(`/api/v1/sources/sync/stream?account_id=${accountId}&force=${force}`, {
      method: 'GET',
      headers,
      signal,
    })
    if (!resp.ok) {
      throw new Error(await resp.text())
    }
    if (!resp.body) {
      throw new Error('浏览器不支持流式响应')
    }

    const reader = resp.body.getReader()
    const decoder = new TextDecoder()
    let buffer = ''
    let doneEvent = false
    let usedCache = false

    const handleEvent = (raw: string) => {
      let event = 'message'
      const data: string[] = []
      for (const line of raw.split(/\r?\n/)) {
        if (line.startsWith('event:')) {
          event = line.slice(6).trim()
          continue
        }
        if (line.startsWith('data:')) {
          data.push(line.slice(5).trimStart())
        }
      }
      if (data.length === 0) return
      const payload = JSON.parse(data.join('\n')) as
        | SyncedPeer
        | SyncedPeer[]
        | { error?: string }
        | { count?: number; used_cache?: boolean }
      if (event === 'peer') {
        onPeers([payload as SyncedPeer])
        return
      }
      if (event === 'peers') {
        onPeers(payload as SyncedPeer[])
        return
      }
      if (event === 'error') {
        throw new Error((payload as { error?: string }).error ?? '同步失败')
      }
      if (event === 'done') {
        usedCache = Boolean((payload as { used_cache?: boolean }).used_cache)
        doneEvent = true
      }
    }

    while (!doneEvent) {
      const { value, done } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const events = buffer.split(/\r?\n\r?\n/)
      buffer = events.pop() ?? ''
      for (const event of events) {
        handleEvent(event)
        if (doneEvent) {
          await reader.cancel()
          break
        }
      }
    }
    if (buffer.trim()) {
      handleEvent(buffer)
    }
    return { usedCache }
  },
  start: (id: number) => http.post(`/sources/${id}/start`),
  stop: (id: number) => http.post(`/sources/${id}/stop`),
}

// --- Templates ---
export const templatesApi = {
  list: () => http.get<ApiList<Template>>('/templates').then((r) => r.data.data),
  preview: (body: Record<string, unknown>) =>
    http.post<ApiItem<TemplatePreview>>('/templates/preview', body).then((r) => r.data.data),
  create: (body: Record<string, unknown>) =>
    http.post<ApiItem<Template>>('/templates', body).then((r) => r.data.data),
  update: (id: number, body: Record<string, unknown>) =>
    http.put<ApiItem<Template>>(`/templates/${id}`, body).then((r) => r.data.data),
  remove: (id: number) => http.delete(`/templates/${id}`),
}

// --- Rules ---
export const rulesApi = {
  list: () => http.get<ApiList<Rule>>('/rules').then((r) => r.data.data),
  meta: () => http.get<ApiItem<RuleMeta>>('/rules/meta').then((r) => r.data.data),
  get: (id: number) => http.get<ApiItem<Rule>>(`/rules/${id}`).then((r) => r.data.data),
  create: (body: Record<string, unknown>) =>
    http.post<ApiItem<Rule>>('/rules', body).then((r) => r.data.data),
  update: (id: number, body: Record<string, unknown>) =>
    http.put<ApiItem<Rule>>(`/rules/${id}`, body).then((r) => r.data.data),
  preview: (body: Record<string, unknown>) =>
    http.post<ApiItem<RulePreviewResult>>('/rules/preview', body).then((r) => r.data.data),
  remove: (id: number) => http.delete(`/rules/${id}`),
}

// --- Deliveries ---
export const deliveriesApi = {
  list: (status = '', limit = 50, offset = 0, filters: Record<string, unknown> = {}) =>
    http
      .get<ApiList<Delivery>>('/deliveries', { params: { status, limit, offset, ...filters } })
      .then((r) => r.data.data),
  page: (status = '', limit = 50, offset = 0, filters: Record<string, unknown> = {}) =>
    http
      .get<ApiPage<Delivery>>('/deliveries', { params: { status, limit, offset, ...filters } })
      .then((r) => ({ data: r.data.data, total: r.data.total ?? r.data.data.length })),
  get: (id: number) => http.get<ApiItem<Delivery>>(`/deliveries/${id}`).then((r) => r.data.data),
  retry: (id: number) => http.post(`/deliveries/${id}/retry`),
  retryDead: () => http.post<{ requeued: number }>('/deliveries/retry-dead').then((r) => r.data),
}

// --- Tokens ---
export const tokensApi = {
  list: () => http.get<ApiList<ApiToken>>('/tokens').then((r) => r.data.data),
  create: (name: string) =>
    http.post<ApiItem<TokenCreated>>('/tokens', { name }).then((r) => r.data.data),
  revoke: (id: number) => http.delete(`/tokens/${id}`),
}

// --- Telegram Config ---
export const telegramConfigApi = {
  apps: {
    list: () => http.get<ApiList<TelegramApp>>('/telegram-apps').then((r) => r.data.data),
    create: (body: Record<string, unknown>) =>
      http.post<ApiItem<TelegramApp>>('/telegram-apps', body).then((r) => r.data.data),
    update: (id: number, body: Record<string, unknown>) =>
      http.put<ApiItem<TelegramApp>>(`/telegram-apps/${id}`, body).then((r) => r.data.data),
    remove: (id: number) => http.delete(`/telegram-apps/${id}`),
  },
  proxies: {
    list: () => http.get<ApiList<SharedProxy>>('/proxies').then((r) => r.data.data),
    create: (body: Record<string, unknown>) =>
      http.post<ApiItem<SharedProxy>>('/proxies', body).then((r) => r.data.data),
    update: (id: number, body: Record<string, unknown>) =>
      http.put<ApiItem<SharedProxy>>(`/proxies/${id}`, body).then((r) => r.data.data),
    remove: (id: number) => http.delete(`/proxies/${id}`),
  },
}

// --- 系统设置 ---
export const settingsApi = {
  media: {
    get: () => http.get<ApiItem<MediaSettings>>('/settings/media').then((r) => r.data.data),
    update: (body: MediaSettingsRequest) =>
      http.put<ApiItem<MediaSettings>>('/settings/media', body).then((r) => r.data.data),
    testS3: (body: MediaSettingsRequest) =>
      http.post<ApiItem<MediaS3TestResult>>('/settings/media/test-s3', body).then((r) => r.data.data),
  },
}
