<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
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

const flow = ref<LoginFlow | null>(null)
const code = ref('')
const password = ref('')
const loading = ref(false)
const serverError = ref('')

// 根据 flow 状态推导当前展示步骤。
const step = computed<'start' | 'code' | 'password' | 'done'>(() => {
  const s = flow.value?.status
  if (s === 'code_required') return 'code'
  if (s === 'password_required') return 'password'
  if (s === 'authorized') return 'done'
  return 'start'
})

// 过期/失败时给出提示并允许重新发送。
const restartHint = computed(() => {
  const s = flow.value?.status
  if (s === 'expired') return '登录流程已过期，请重新发送验证码。'
  if (s === 'failed') return flow.value?.last_error || '上次登录失败，请重试。'
  return ''
})

watch(
  () => props.show,
  (open) => {
    if (open) {
      code.value = ''
      password.value = ''
      serverError.value = ''
      flow.value = null
      void refresh()
    }
  },
)

// 打开弹窗时恢复该账号未完成的登录 flow。
async function refresh() {
  if (!props.account) return
  loading.value = true
  try {
    flow.value = await accountsApi.login.status(props.account.id)
  } catch (e) {
    message.error('加载登录状态失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

function handleErr(e: unknown) {
  const resp = (e as { response?: { data?: { data?: LoginFlow } } })?.response?.data
  if (resp?.data) flow.value = resp.data
  serverError.value = errText(e)
  message.error(serverError.value)
}

async function start() {
  if (!props.account) return
  loading.value = true
  serverError.value = ''
  try {
    flow.value = await accountsApi.login.start(props.account.id)
    message.success('验证码已发送')
  } catch (e) {
    handleErr(e)
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
    handleErr(e)
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
    handleErr(e)
  } finally {
    loading.value = false
  }
}

function onSuccess() {
  message.success('登录成功，session 已加密保存')
  emit('success')
  close()
}

async function cancel() {
  if (props.account && flow.value && step.value !== 'done') {
    try {
      await accountsApi.login.cancel(props.account.id, flow.value.flow_id)
    } catch {
      // 取消失败不阻断关闭。
    }
  }
  close()
}

function close() {
  code.value = ''
  password.value = ''
  emit('update:show', false)
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="`登录账号：${account?.name ?? ''}`"
    style="width: 460px"
    @update:show="(v: boolean) => emit('update:show', v)"
  >
    <n-spin :show="loading">
      <n-space vertical size="large">
        <n-text depth="3">手机号：{{ account?.phone_number }}</n-text>

        <n-alert v-if="restartHint" type="warning">{{ restartHint }}</n-alert>
        <n-alert v-else-if="serverError" type="error">{{ serverError }}</n-alert>

        <!-- 步骤 1：发送验证码 -->
        <div v-if="step === 'start'">
          <n-button type="primary" block :loading="loading" @click="start">
            {{ flow?.status === 'expired' || flow?.status === 'failed' ? '重新发送验证码' : '发送验证码' }}
          </n-button>
        </div>

        <!-- 步骤 2：输入验证码 -->
        <div v-else-if="step === 'code'">
          <n-space vertical>
            <n-text depth="3">请输入 Telegram 发送的验证码。</n-text>
            <n-input
              v-model:value="code"
              placeholder="验证码"
              @keyup.enter="submitCode"
            />
            <n-space justify="space-between">
              <n-button quaternary size="small" :loading="loading" @click="start">重新发送</n-button>
              <n-button type="primary" :loading="loading" @click="submitCode">确认验证码</n-button>
            </n-space>
          </n-space>
        </div>

        <!-- 步骤 3：两步验证密码 -->
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

        <!-- 完成 -->
        <div v-else>
          <n-alert type="success">登录成功。</n-alert>
        </div>
      </n-space>
    </n-spin>

    <template #footer>
      <n-space justify="end">
        <n-button @click="cancel">{{ step === 'done' ? '关闭' : '取消登录' }}</n-button>
      </n-space>
    </template>
  </n-modal>
</template>
