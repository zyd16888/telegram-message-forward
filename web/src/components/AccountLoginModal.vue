<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import QRCode from 'qrcode'
import { accountsApi } from '@/api/client'
import type { Account, LoginFlow } from '@/types'
import { errText } from '@/utils/error'

const props = defineProps<{
  show: boolean
  account: Account | null
}>()
const emit = defineEmits<{
  'update:show': [value: boolean]
  success: []
}>()

const message = useMessage()

const tab = ref<'code' | 'qr'>('code')

// --- 验证码登录 ---
const flow = ref<LoginFlow | null>(null)
const code = ref('')
const password = ref('')
const loading = ref(false)
const serverError = ref('')

const step = computed<'start' | 'code' | 'password' | 'done'>(() => {
  const s = flow.value?.status
  if (s === 'code_required') return 'code'
  if (s === 'password_required') return 'password'
  if (s === 'authorized') return 'done'
  return 'start'
})

const restartHint = computed(() => {
  const s = flow.value?.status
  if (s === 'expired') return '登录流程已过期，请重新发送验证码。'
  if (s === 'failed') return flow.value?.last_error || '上次登录失败，请重试。'
  return ''
})

// --- 扫码登录 ---
const qrFlow = ref<LoginFlow | null>(null)
const qrDataUrl = ref('')
const qrError = ref('')
const nowMs = ref(Date.now())
let pollTimer: ReturnType<typeof setInterval> | null = null
let tickTimer: ReturnType<typeof setInterval> | null = null

const qrCountdown = computed(() => {
  if (!qrFlow.value?.qr_expires_at) return 0
  const ms = new Date(qrFlow.value.qr_expires_at).getTime() - nowMs.value
  return Math.max(0, Math.ceil(ms / 1000))
})
const qrNeedsRefresh = computed(() => qrFlow.value?.status === 'qr_refresh_required')

watch(
  () => props.show,
  (open) => {
    if (open) {
      resetState()
      void recover()
    } else {
      stopTimers()
    }
  },
)

watch(tab, (t) => {
  if (!props.show) return
  if (t === 'qr') {
    if (!qrFlow.value || isTerminal(qrFlow.value)) void qrStart()
    else startTimers()
  } else {
    stopTimers()
  }
})

onUnmounted(stopTimers)

function isTerminal(f: LoginFlow) {
  return ['authorized', 'failed', 'cancelled', 'expired'].includes(f.status)
}

function resetState() {
  tab.value = 'code'
  code.value = ''
  password.value = ''
  serverError.value = ''
  qrError.value = ''
  flow.value = null
  qrFlow.value = null
  qrDataUrl.value = ''
}

// 打开弹窗时恢复未完成的登录 flow（验证码或扫码）。
async function recover() {
  if (!props.account) return
  loading.value = true
  try {
    const active = await accountsApi.login.status(props.account.id)
    if (active && !isTerminal(active)) {
      if (active.method === 'qr') {
        tab.value = 'qr'
        qrFlow.value = active
        await renderQR()
        startTimers()
      } else {
        tab.value = 'code'
        flow.value = active
      }
    }
  } catch (e) {
    message.error('加载登录状态失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

function handleErr(e: unknown, target: 'code' | 'qr') {
  const resp = (e as { response?: { data?: { data?: LoginFlow } } })?.response?.data
  if (resp?.data) {
    if (target === 'qr') qrFlow.value = resp.data
    else flow.value = resp.data
  }
  const msg = errText(e)
  if (target === 'qr') qrError.value = msg
  else serverError.value = msg
  message.error(msg)
}

// --- 验证码登录动作 ---
async function start() {
  if (!props.account) return
  loading.value = true
  serverError.value = ''
  try {
    flow.value = await accountsApi.login.start(props.account.id)
    message.success('验证码已发送')
  } catch (e) {
    handleErr(e, 'code')
  } finally {
    loading.value = false
  }
}

async function submitCode() {
  if (!props.account || !flow.value) return
  if (!code.value.trim()) {
    message.warning('请输入验证码')
    return
  }
  loading.value = true
  serverError.value = ''
  try {
    flow.value = await accountsApi.login.code(props.account.id, flow.value.flow_id, code.value.trim())
    if (flow.value.status === 'authorized') onSuccess()
    else if (flow.value.status === 'password_required') message.info('该账号已开启两步验证，请输入密码')
  } catch (e) {
    handleErr(e, 'code')
  } finally {
    loading.value = false
  }
}

async function submitPassword() {
  if (!props.account || !flow.value) return
  if (!password.value) {
    message.warning('请输入两步验证密码')
    return
  }
  loading.value = true
  serverError.value = ''
  try {
    flow.value = await accountsApi.login.password(props.account.id, flow.value.flow_id, password.value)
    if (flow.value.status === 'authorized') onSuccess()
  } catch (e) {
    handleErr(e, 'code')
  } finally {
    loading.value = false
  }
}

// --- 扫码登录动作 ---
async function qrStart() {
  if (!props.account) return
  loading.value = true
  qrError.value = ''
  try {
    qrFlow.value = await accountsApi.login.qr.start(props.account.id)
    await renderQR()
    startTimers()
  } catch (e) {
    handleErr(e, 'qr')
  } finally {
    loading.value = false
  }
}

async function qrPoll() {
  if (!props.account || !qrFlow.value) return
  try {
    const f = await accountsApi.login.qr.status(props.account.id, qrFlow.value.flow_id)
    if (!f) return
    qrFlow.value = f
    if (f.status === 'authorized') {
      onSuccess()
      return
    }
    if (f.status === 'qr_refresh_required') {
      await qrRefresh()
      return
    }
    await renderQR()
  } catch (e) {
    handleErr(e, 'qr')
  }
}

async function qrRefresh() {
  if (!props.account || !qrFlow.value) return
  qrError.value = ''
  try {
    qrFlow.value = await accountsApi.login.qr.refresh(props.account.id, qrFlow.value.flow_id)
    await renderQR()
    startTimers()
  } catch (e) {
    handleErr(e, 'qr')
  }
}

async function renderQR() {
  if (qrFlow.value?.qr_url) {
    try {
      qrDataUrl.value = await QRCode.toDataURL(qrFlow.value.qr_url, { width: 220, margin: 1 })
    } catch {
      qrDataUrl.value = ''
    }
  }
}

function startTimers() {
  stopTimers()
  pollTimer = setInterval(qrPoll, 3000)
  tickTimer = setInterval(() => {
    nowMs.value = Date.now()
  }, 1000)
}

function stopTimers() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  if (tickTimer) {
    clearInterval(tickTimer)
    tickTimer = null
  }
}

// --- 通用 ---
function onSuccess() {
  stopTimers()
  message.success('登录成功，session 已加密保存')
  emit('success')
  close()
}

async function cancel() {
  stopTimers()
  if (props.account) {
    try {
      if (flow.value && step.value !== 'done') {
        await accountsApi.login.cancel(props.account.id, flow.value.flow_id)
      }
      if (qrFlow.value && !isTerminal(qrFlow.value)) {
        await accountsApi.login.qr.cancel(props.account.id, qrFlow.value.flow_id)
      }
    } catch {
      // 取消失败不阻断关闭。
    }
  }
  close()
}

function close() {
  stopTimers()
  emit('update:show', false)
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="`登录账号：${account?.name ?? ''}`"
    style="width: 460px"
    @update:show="(v: boolean) => (v ? undefined : cancel())"
  >
    <n-text depth="3">手机号：{{ account?.phone_number }}</n-text>

    <n-tabs v-model:value="tab" type="line" animated class="mt">
      <!-- 验证码登录 -->
      <n-tab-pane name="code" tab="验证码登录">
        <n-spin :show="loading">
          <n-space vertical size="large">
            <n-alert v-if="restartHint" type="warning">{{ restartHint }}</n-alert>
            <n-alert v-else-if="serverError" type="error">{{ serverError }}</n-alert>

            <div v-if="step === 'start'">
              <n-button type="primary" block :loading="loading" @click="start">
                {{ flow?.status === 'expired' || flow?.status === 'failed' ? '重新发送验证码' : '发送验证码' }}
              </n-button>
            </div>

            <div v-else-if="step === 'code'">
              <n-space vertical>
                <n-text depth="3">请输入 Telegram 发送的验证码。</n-text>
                <n-input v-model:value="code" placeholder="验证码" @keyup.enter="submitCode" />
                <n-space justify="space-between">
                  <n-button quaternary size="small" :loading="loading" @click="start">重新发送</n-button>
                  <n-button type="primary" :loading="loading" @click="submitCode">确认验证码</n-button>
                </n-space>
              </n-space>
            </div>

            <div v-else-if="step === 'password'">
              <n-space vertical>
                <n-text depth="3">该账号已开启两步验证，请输入密码。</n-text>
                <n-input
                  v-model:value="password"
                  type="password"
                  show-password-on="click"
                  placeholder="两步验证密码"
                  @keyup.enter="submitPassword"
                />
                <n-button type="primary" block :loading="loading" @click="submitPassword">
                  确认密码
                </n-button>
              </n-space>
            </div>

            <div v-else>
              <n-alert type="success">登录成功。</n-alert>
            </div>
          </n-space>
        </n-spin>
      </n-tab-pane>

      <!-- 扫码登录 -->
      <n-tab-pane name="qr" tab="扫码登录">
        <n-space vertical align="center" size="large">
          <n-alert v-if="qrError" type="error" style="width: 100%">{{ qrError }}</n-alert>

          <div v-if="qrNeedsRefresh" class="qr-box">
            <n-empty description="二维码已失效">
              <template #extra>
                <n-button size="small" type="primary" @click="qrRefresh">刷新二维码</n-button>
              </template>
            </n-empty>
          </div>
          <div v-else-if="qrDataUrl" class="qr-box">
            <img :src="qrDataUrl" alt="Telegram 登录二维码" width="220" height="220" />
          </div>
          <n-spin v-else />

          <n-text depth="3">
            用手机 Telegram 打开「设置 → 设备 → 关联桌面设备」扫描二维码授权登录。
          </n-text>
          <n-space align="center">
            <n-text v-if="!qrNeedsRefresh && qrCountdown > 0" depth="3">
              {{ qrCountdown }} 秒后自动刷新
            </n-text>
            <n-button size="small" quaternary :loading="loading" @click="qrRefresh">手动刷新</n-button>
          </n-space>
        </n-space>
      </n-tab-pane>
    </n-tabs>

    <template #footer>
      <n-space justify="end">
        <n-button @click="cancel">{{ step === 'done' ? '关闭' : '取消登录' }}</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<style scoped>
.mt {
  margin-top: 12px;
}
.qr-box {
  width: 220px;
  height: 220px;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
