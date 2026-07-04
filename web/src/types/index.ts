// API 响应类型，与后端 internal/api/dto 对应。

export interface ApiList<T> {
  data: T[]
  total?: number
}

export interface ApiItem<T> {
  data: T
}

export interface ApiPage<T> {
  data: T[]
  total: number
}

export interface Proxy {
  type?: string
  addr?: string
  username?: string
  has_password: boolean
}

export interface Account {
  id: number
  name: string
  phone_number: string
  telegram_app_id?: number
  proxy_id?: number
  app_id: number
  status: string
  proxy: Proxy
  last_login_at?: string
  last_error?: string
  created_at: string
  updated_at: string
}

export interface MediaCapability {
  type: string
  supported: boolean
  max_size_mb?: number
  supports_public_url: boolean
  requires_upload: boolean
  supports_binary: boolean
  delivery_mode?: string
  fallback?: string
}

export interface Capabilities {
  supports_text: boolean
  supports_markdown: boolean
  supports_html: boolean
  supports_image: boolean
  supports_file: boolean
  supports_audio: boolean
  supports_video: boolean
  max_text_length?: number
  max_file_size_mb?: number
  media?: MediaCapability[]
  notes?: string[]
}

export type FieldType =
  | 'text'
  | 'password'
  | 'textarea'
  | 'number'
  | 'boolean'
  | 'select'
  | 'multi_select'
  | 'string_list'
  | 'key_value'

export interface FieldOption {
  label: string
  value: string
}

export interface FieldSpec {
  key: string
  label: string
  type: FieldType
  required?: boolean
  secret?: boolean
  default?: unknown
  placeholder?: string
  help?: string
  options?: FieldOption[]
  min?: number
  max?: number
}

export interface SinkDescriptor {
  type: string
  label: string
  description?: string
  config_fields: FieldSpec[]
  secret_field?: FieldSpec
  capabilities: Capabilities
}

export interface SinkTestResult {
  success: boolean
  error?: string
  response_summary?: unknown
}

export interface Sink {
  id: number
  type: string
  name: string
  enabled: boolean
  config: Record<string, unknown>
  capabilities: Capabilities
  observability: {
    last_test_at?: string
    last_test_success: boolean
    last_test_error?: string
    recent_failure?: string
    delivery_total_24h: number
    delivery_success_24h: number
    success_rate_24h: number
  }
  has_secret: boolean
  created_at: string
  updated_at: string
}

export interface Source {
  id: number
  type: string
  account_id: number
  peer_type: string
  peer_id: number
  name: string
  username?: string
  enabled: boolean
  config: Record<string, unknown>
  last_message_id: number
  last_synced_at?: string
  runner_status?: string
  runner_subscriptions?: number
  runner_recent_message_at?: string
  runner_last_error?: string
  created_at: string
  updated_at: string
}

export interface SyncedPeer {
  peer_type: string
  peer_kind: string
  peer_id: number
  name: string
  username?: string
  display_type: string
  is_bot: boolean
  is_channel: boolean
  is_supergroup: boolean
  is_forum: boolean
  flags?: string[]
  cached?: boolean
}

export interface Template {
  id: number
  name: string
  format: string
  content: string
  created_at: string
  updated_at: string
}

export interface TemplatePreview {
  format: string
  text: string
}

export interface ConditionConfig {
  type: string
  config: Record<string, unknown>
}

export interface ProcessorConfig {
  type: string
  config: Record<string, unknown>
}

export interface RuleItemDescriptor {
  type: string
  label: string
  description?: string
  fields: FieldSpec[]
}

export interface RuleMeta {
  conditions: RuleItemDescriptor[]
  processors: RuleItemDescriptor[]
}

export interface RuleTarget {
  sink_id: number
  template_id?: number
}

export interface RulePreviewResult {
  matched: boolean
  processed_text: string
  media?: Array<{
    type: string
    file_name?: string
    mime_type?: string
    size?: number
    caption?: string
  }>
  targets: RuleTarget[]
}

export interface Rule {
  id: number
  name: string
  enabled: boolean
  priority: number
  conditions: ConditionConfig[]
  processors: ProcessorConfig[]
  stop_on_match: boolean
  source_ids: number[]
  targets: RuleTarget[]
  created_at: string
  updated_at: string
}

export interface RuleInitialDraft {
  name?: string
  source_ids?: number[]
  targets?: RuleTarget[]
}

export interface Delivery {
  id: number
  message_id: number
  rule_id: number
  sink_id: number
  template_id?: number
  status: string
  attempt_count: number
  max_attempts: number
  next_retry_at?: string
  last_error?: string
  message_text?: string
  message_type?: string
  sender_name?: string
  source_name?: string
  source_username?: string
  source_peer_type?: string
  sink_name?: string
  sink_type?: string
  rule_name?: string
  template_name?: string
  attempts?: DeliveryAttempt[]
  created_at: string
  updated_at: string
}

export interface DeliveryAttempt {
  id: number
  delivery_task_id: number
  attempt_no: number
  status: string
  request_summary?: unknown
  response_summary?: unknown
  error?: string
  started_at?: string
  finished_at?: string
  created_at: string
}

export interface ApiToken {
  id: number
  name: string
  revoked: boolean
  created_at: string
  last_used_at?: string
  revoked_at?: string
}

export interface TokenCreated {
  id: number
  name: string
  token: string
}

export interface LoginFlow {
  flow_id: string
  account_id: number
  method: string
  status: string
  current_step: string
  expires_at: string
  qr_url?: string
  qr_expires_at?: string
  last_error?: string
  completed_at?: string
}

export interface BootstrapStatus {
  auth_enabled: boolean
  can_bootstrap: boolean
}

export interface Me {
  auth_enabled: boolean
  authenticated: boolean
  can_bootstrap: boolean
  username?: string
  credential_type?: string
}

export interface TelegramApp {
  id: number
  name: string
  app_id: number
  enabled: boolean
  has_hash: boolean
  created_at: string
  updated_at: string
}

export interface SharedProxy {
  id: number
  name: string
  type: string
  addr: string
  username?: string
  enabled: boolean
  has_password: boolean
  created_at: string
  updated_at: string
}

// --- 系统设置（对应 internal/api/dto/settings.go） ---

export interface MediaS3Settings {
  enabled: boolean
  endpoint: string
  region: string
  bucket: string
  access_key: string
  use_ssl: boolean
  key_prefix: string
  public_base_url: string
  auto_cleanup: boolean
}

export interface MediaDownloadSettings {
  image_max_mb: number
  file_max_mb: number
  // 文件扩展名白名单（不带点）；空列表表示不限类型。
  file_types: string[]
}

export interface MediaSettings {
  dir: string
  public_base_url: string
  url_ttl_hours: number
  retention_hours: number
  download: MediaDownloadSettings
  s3: MediaS3Settings
  has_s3_secret: boolean
  // database：页面已保存；file：仍在使用配置文件默认值。
  source: 'database' | 'file'
}

export interface MediaSettingsRequest {
  dir: string
  public_base_url: string
  url_ttl_hours: number
  retention_hours: number
  download: MediaDownloadSettings
  s3: MediaS3Settings
  // 省略表示保留已有 secret。
  s3_secret_key?: string
}

export interface MediaS3TestResult {
  success: boolean
  error?: string
}
