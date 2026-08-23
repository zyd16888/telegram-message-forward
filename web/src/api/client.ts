import axios, { type AxiosInstance } from 'axios'
import type {
  Account,
  ApiItem,
  ApiList,
  ApiPage,
  ApiToken,
  BootstrapStatus,
  Delivery,
  Filter,
  FilterRequest,
  LoginFlow,
  Me,
  Flow,
  FlowNode,
  FlowRequest,
  LinearFlow,
  RuleMeta,
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
  MediaCleanupRequest,
  MediaCleanupResult,
  DataRetentionSettings,
  DataRetentionRequest,
  DataCleanupRequest,
  DataCleanupResult,
  AIProvider,
  AIProviderRequest,
  AIProviderTestResult,
  AIDigestPreset,
  AIDigestOutputTemplate,
  AIDigestOutputTemplateRequest,
  AIDigestProfile,
  AIDigestProfileRequest,
  AIDigestRun,
  AIDigestRunDetail,
  CursorPage,
  MessageDetail,
  MessageItem,
  BackupPreview,
  BackupRestoreResult,
  DashboardSummary,
  ChatArchive,
  ChatArchiveMessage,
  ChatExportJob,
  ChatExportCreateRequest,
  ChatExportCreated,
} from '@/types'

const TOKEN_KEY = 'tmf_api_token'

type UnauthorizedHandler = () => void | Promise<void>

let unauthorizedHandler: UnauthorizedHandler | null = null

export function setUnauthorizedHandler(handler: UnauthorizedHandler | null): void {
  unauthorizedHandler = handler
}

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

// 会话失效（401）时清理本地凭证；跳转由应用入口注入，避免 API 层依赖 router。
http.interceptors.response.use(
  (resp) => resp,
  (error) => {
    const url: string = error?.config?.url ?? ''
    const status: number | undefined = error?.response?.status
    if (status === 401 && !url.startsWith('/auth/')) {
      setToken('')
      void unauthorizedHandler?.()
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
  previewHistory: (id: number, limit = 50) =>
    http
      .post<ApiItem<{ items: Array<Record<string, unknown>>; fetched: number; max_message_id: number }>>(
        `/sources/${id}/history/preview`,
        { limit },
      )
      .then((r) => r.data.data),
  backfillHistory: (id: number, limit = 50) =>
    http
      .post<
        ApiItem<{ items: Array<Record<string, unknown>>; fetched: number; ingested: number; max_message_id: number }>
      >(`/sources/${id}/history/backfill`, { limit }, { timeout: 120000 })
      .then((r) => r.data.data),
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

export function flowToLinearFlow(flow: Flow): LinearFlow {
  const byType = (type: FlowNode['type']) =>
    flow.nodes
      .filter((node) => node.type === type)
      .sort((a, b) => a.pos_x - b.pos_x || a.pos_y - b.pos_y || a.id - b.id)
  const filterNodes = byType('filter')
  const filterIDs = filterNodes.flatMap((node) => node.config.filter_ids ?? [])
  return {
    id: flow.id,
    name: flow.name,
    enabled: flow.enabled,
    priority: flow.priority,
    stop_on_match: flow.stop_on_match,
    source_ids: byType('source')
      .map((node) => node.ref_id ?? 0)
      .filter((id) => id > 0),
    filter_ids: filterIDs,
    conditions: filterIDs.length ? [] : filterNodes.flatMap((node) => node.config.conditions ?? []),
    processors: byType('processor').flatMap((node) => node.config.processors ?? []),
    targets: byType('target')
      .map((node) => ({ sink_id: node.ref_id ?? 0, template_id: node.template_id }))
      .filter((target) => target.sink_id > 0),
    created_at: flow.created_at,
    updated_at: flow.updated_at,
  }
}

// --- Flows ---
export const flowsApi = {
  list: () => http.get<ApiList<Flow>>('/flows').then((r) => r.data.data),
  meta: () => http.get<ApiItem<RuleMeta>>('/flows/meta').then((r) => r.data.data),
  get: (id: number) => http.get<ApiItem<Flow>>(`/flows/${id}`).then((r) => r.data.data),
  create: (body: FlowRequest) => http.post<ApiItem<Flow>>('/flows', body).then((r) => r.data.data),
  update: (id: number, body: FlowRequest) => http.put<ApiItem<Flow>>(`/flows/${id}`, body).then((r) => r.data.data),
  remove: (id: number) => http.delete(`/flows/${id}`),
}

// --- Filters（可复用过滤器） ---
export const filtersApi = {
  list: () => http.get<ApiList<Filter>>('/filters').then((r) => r.data.data),
  get: (id: number) => http.get<ApiItem<Filter>>(`/filters/${id}`).then((r) => r.data.data),
  create: (body: FilterRequest) => http.post<ApiItem<Filter>>('/filters', body).then((r) => r.data.data),
  update: (id: number, body: FilterRequest) =>
    http.put<ApiItem<Filter>>(`/filters/${id}`, body).then((r) => r.data.data),
  remove: (id: number) => http.delete(`/filters/${id}`),
}

// --- Dashboard ---
export const dashboardApi = {
  summary: (sinceHours = 24) =>
    http
      .get<ApiItem<DashboardSummary>>('/dashboard/summary', { params: { since_hours: sinceHours } })
      .then((r) => r.data.data),
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

// --- Messages ---
export const messagesApi = {
  page: (params: Record<string, unknown> = {}) =>
    http.get<CursorPage<MessageItem>>('/messages', { params }).then((r) => r.data),
  get: (id: number) => http.get<ApiItem<MessageDetail>>(`/messages/${id}`).then((r) => r.data.data),
}

// --- 聊天归档 ---
export const chatArchiveApi = {
  listArchives: () => http.get<ApiList<ChatArchive>>('/chat-archives').then((r) => r.data.data),
  getArchive: (id: number) =>
    http.get<ApiItem<ChatArchive>>(`/chat-archives/${id}`).then((r) => r.data.data),
  removeArchive: (id: number) => http.delete(`/chat-archives/${id}`),
  searchMessages: (id: number, params: Record<string, unknown> = {}) =>
    http
      .get<CursorPage<ChatArchiveMessage>>(`/chat-archives/${id}/messages`, { params })
      .then((r) => r.data),

  listJobs: (archiveId: number) =>
    http
      .get<ApiList<ChatExportJob>>('/chat-exports', { params: { archive_id: archiveId } })
      .then((r) => r.data.data),
  getJob: (id: number) =>
    http.get<ApiItem<ChatExportJob>>(`/chat-exports/${id}`).then((r) => r.data.data),
  createJob: (body: ChatExportCreateRequest) =>
    http.post<ChatExportCreated>('/chat-exports', body).then((r) => r.data),
  cancelJob: (id: number) => http.post(`/chat-exports/${id}/cancel`),

  /**
   * 下载导出文件。
   *
   * 走 blob 而不是直接开新窗口：下载接口需要 Authorization 头，
   * 而 <a href> / window.open 带不上 token。
   */
  download: async (id: number, params: Record<string, unknown>) => {
    const resp = await http.get(`/chat-archives/${id}/download`, {
      params,
      responseType: 'blob',
    })
    const disposition = String(resp.headers['content-disposition'] ?? '')
    const matched = /filename="?([^"]+)"?/.exec(disposition)
    const name = matched?.[1] ?? `chat-archive-${id}.txt`
    const url = URL.createObjectURL(resp.data as Blob)
    try {
      const a = document.createElement('a')
      a.href = url
      a.download = name
      document.body.appendChild(a)
      a.click()
      a.remove()
    } finally {
      URL.revokeObjectURL(url)
    }
    return name
  },
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
    cleanup: (body: MediaCleanupRequest) =>
      http.post<ApiItem<MediaCleanupResult>>('/settings/media/cleanup', body).then((r) => r.data.data),
  },
  dataRetention: {
    get: () =>
      http.get<ApiItem<DataRetentionSettings>>('/settings/data-retention').then((r) => r.data.data),
    update: (body: DataRetentionRequest) =>
      http.put<ApiItem<DataRetentionSettings>>('/settings/data-retention', body).then((r) => r.data.data),
    cleanup: (body: DataCleanupRequest) =>
      http.post<ApiItem<DataCleanupResult>>('/settings/data-retention/cleanup', body).then((r) => r.data.data),
  },
  backups: {
    export: (password: string, includeSessions: boolean) =>
      http.post<Blob>('/settings/backups/export', { password, include_sessions: includeSessions }, { responseType: 'blob', timeout: 120000 }),
    inspect: (file: File, password: string) => {
      const body = new FormData()
      body.append('file', file)
      body.append('password', password)
      return http.post<ApiItem<BackupPreview>>('/settings/backups/inspect', body, { timeout: 120000 }).then((r) => r.data.data)
    },
    restore: (file: File, password: string) => {
      const body = new FormData()
      body.append('file', file)
      body.append('password', password)
      body.append('confirmed', 'true')
      return http.post<ApiItem<BackupRestoreResult>>('/settings/backups/restore', body, { timeout: 120000 }).then((r) => r.data.data)
    },
  },
}

// --- AI 整理 ---
export const aiApi = {
  provider: {
    get: () => http.get<ApiItem<AIProvider>>('/ai/provider').then((r) => r.data.data),
    update: (body: AIProviderRequest) =>
      http.put<ApiItem<AIProvider>>('/ai/provider', body).then((r) => r.data.data),
    test: () => http.post<ApiItem<AIProviderTestResult>>('/ai/provider/test').then((r) => r.data.data),
  },
  providers: {
    list: () => http.get<ApiList<AIProvider>>('/ai/providers').then((r) => r.data.data),
    create: (body: AIProviderRequest) =>
      http.post<ApiItem<AIProvider>>('/ai/providers', body).then((r) => r.data.data),
    get: (id: string) => http.get<ApiItem<AIProvider>>(`/ai/providers/${id}`).then((r) => r.data.data),
    update: (id: string, body: AIProviderRequest) =>
      http.put<ApiItem<AIProvider>>(`/ai/providers/${id}`, body).then((r) => r.data.data),
    remove: (id: string) => http.delete(`/ai/providers/${id}`),
    test: (id: string, body?: AIProviderRequest) =>
      http.post<ApiItem<AIProviderTestResult>>(`/ai/providers/${id}/test`, body).then((r) => r.data.data),
    testDraft: (body: AIProviderRequest) =>
      http.post<ApiItem<AIProviderTestResult>>('/ai/providers/test', body).then((r) => r.data.data),
  },
  presets: {
    list: () => http.get<ApiList<AIDigestPreset>>('/ai/presets').then((r) => r.data.data),
  },
  outputTemplates: {
    list: () => http.get<ApiList<AIDigestOutputTemplate>>('/ai/output-templates').then((r) => r.data.data),
    get: (id: number) =>
      http.get<ApiItem<AIDigestOutputTemplate>>(`/ai/output-templates/${id}`).then((r) => r.data.data),
    create: (body: AIDigestOutputTemplateRequest) =>
      http.post<ApiItem<AIDigestOutputTemplate>>('/ai/output-templates', body).then((r) => r.data.data),
    update: (id: number, body: AIDigestOutputTemplateRequest) =>
      http.put<ApiItem<AIDigestOutputTemplate>>(`/ai/output-templates/${id}`, body).then((r) => r.data.data),
    remove: (id: number) => http.delete(`/ai/output-templates/${id}`),
  },
  profiles: {
    list: () => http.get<ApiList<AIDigestProfile>>('/ai/digests').then((r) => r.data.data),
    get: (id: number) => http.get<ApiItem<AIDigestProfile>>(`/ai/digests/${id}`).then((r) => r.data.data),
    create: (body: AIDigestProfileRequest) =>
      http.post<ApiItem<AIDigestProfile>>('/ai/digests', body).then((r) => r.data.data),
    update: (id: number, body: AIDigestProfileRequest) =>
      http.put<ApiItem<AIDigestProfile>>(`/ai/digests/${id}`, body).then((r) => r.data.data),
    remove: (id: number) => http.delete(`/ai/digests/${id}`),
    previewDraft: (body: AIDigestProfileRequest) =>
      http.post<ApiItem<AIDigestRunDetail>>('/ai/digests/preview', body, { timeout: 180000 }).then((r) => r.data.data),
    preview: (id: number) =>
      http.post<ApiItem<AIDigestRunDetail>>(`/ai/digests/${id}/preview`, undefined, { timeout: 180000 }).then((r) => r.data.data),
    run: (id: number) =>
      http.post<ApiItem<AIDigestRunDetail>>(`/ai/digests/${id}/run`, undefined, { timeout: 180000 }).then((r) => r.data.data),
    runs: (id: number, limit = 30, offset = 0) =>
      http.get<ApiPage<AIDigestRun>>(`/ai/digests/${id}/runs`, { params: { limit, offset } }).then((r) => r.data),
  },
  runs: {
    get: (id: number) => http.get<ApiItem<AIDigestRunDetail>>(`/ai/runs/${id}`).then((r) => r.data.data),
    cloneProfile: (id: number) =>
      http.post<ApiItem<AIDigestProfile>>(`/ai/runs/${id}/clone-profile`).then((r) => r.data.data),
    cancel: (id: number) => http.post(`/ai/runs/${id}/cancel`),
    deliver: (id: number) =>
      http.post<ApiItem<{ delivery_task_ids: number[] }>>(`/ai/runs/${id}/deliver`).then((r) => r.data.data),
    cleanup: (retentionDays = 30) =>
      http.post<ApiItem<{ deleted: number }>>('/ai/runs/cleanup', undefined, { params: { retention_days: retentionDays } }).then((r) => r.data.data),
  },
}
