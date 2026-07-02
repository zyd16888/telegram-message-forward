// API 响应类型，与后端 internal/api/dto 对应。

export interface ApiList<T> {
  data: T[]
}

export interface ApiItem<T> {
  data: T
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
  app_id: number
  status: string
  proxy: Proxy
  last_login_at?: string
  last_error?: string
  created_at: string
  updated_at: string
}

export interface Capabilities {
  supports_text: boolean
  supports_markdown: boolean
  supports_html: boolean
  supports_image: boolean
  supports_file: boolean
  max_text_length?: number
  max_file_size_mb?: number
}

export interface Sink {
  id: number
  type: string
  name: string
  enabled: boolean
  config: Record<string, unknown>
  capabilities: Capabilities
  has_secret: boolean
  created_at: string
  updated_at: string
}

export interface Source {
  id: number
  account_id: number
  peer_type: string
  peer_id: number
  name: string
  username?: string
  enabled: boolean
  config: Record<string, unknown>
  last_message_id: number
  last_synced_at?: string
  created_at: string
  updated_at: string
}

export interface SyncedPeer {
  peer_type: string
  peer_id: number
  name: string
  username?: string
}

export interface Template {
  id: number
  name: string
  format: string
  content: string
  created_at: string
  updated_at: string
}

export interface ConditionConfig {
  type: string
  config?: Record<string, unknown>
}

export interface ProcessorConfig {
  type: string
  config?: Record<string, unknown>
}

export interface RuleTarget {
  sink_id: number
  template_id?: number
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
  created_at: string
  updated_at: string
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
}
