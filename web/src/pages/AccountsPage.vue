<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NSpace, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { accountsApi, telegramConfigApi } from '@/api/client'
import type { Account, SharedProxy, TelegramApp } from '@/types'
import { errText } from '@/utils/error'
import AccountLoginModal from '@/components/AccountLoginModal.vue'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'

const message = useMessage()
const dialog = useDialog()
const router = useRouter()

const accounts = ref<Account[]>([])
const apps = ref<TelegramApp[]>([])
const proxies = ref<SharedProxy[]>([])
const loading = ref(false)
const showCreate = ref(false)
const showLogin = ref(false)
const loginAccount = ref<Account | null>(null)

const form = ref({
  name: '',
  phone_number: '',
  telegram_app_id: null as number | null,
  proxy_id: null as number | null,
})

const appOptions = computed(() =>
  apps.value
    .filter((app) => app.enabled)
    .map((app) => ({ label: `${app.name} (${app.app_id})`, value: app.id })),
)

const proxyOptions = computed(() => [
  { label: '不使用代理', value: 0 },
  ...proxies.value
    .filter((proxy) => proxy.enabled)
    .map((proxy) => ({ label: `${proxy.name} (${proxy.type} ${proxy.addr})`, value: proxy.id })),
])

const statusType: Record<string, 'success' | 'warning' | 'error' | 'default'> = {
  active: 'success',
  logging_in: 'warning',
  inactive: 'default',
  error: 'error',
  banned: 'error',
}

async function load() {
  loading.value = true
  try {
    accounts.value = await accountsApi.list()
    apps.value = await telegramConfigApi.apps.list()
    proxies.value = await telegramConfigApi.proxies.list()
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

async function create() {
  if (!form.value.telegram_app_id) {
    message.warning('请先选择 Telegram App')
    return
  }
  try {
    const body: Record<string, unknown> = {
      name: form.value.name,
      phone_number: form.value.phone_number,
      telegram_app_id: form.value.telegram_app_id,
    }
    if (form.value.proxy_id) body.proxy_id = form.value.proxy_id
    const created = await accountsApi.create(body)
    message.success('账号已创建，点击「登录」完成 Telegram 登录')
    showCreate.value = false
    // 新建后自动打开登录弹窗。
    openLogin(created)
    form.value = { name: '', phone_number: '', telegram_app_id: null, proxy_id: null }
    await load()
  } catch (e) {
    message.error('创建失败：' + errText(e))
  }
}

function confirmDelete(row: Account) {
  dialog.warning({
    title: '删除账号',
    content: `确定删除账号「${row.name}」？相关来源与消息将级联删除。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await accountsApi.remove(row.id)
        message.success('已删除')
        await load()
      } catch (e) {
        message.error('删除失败：' + errText(e))
      }
    },
  })
}

function openLogin(row: Account) {
  loginAccount.value = row
  showLogin.value = true
}

async function onLoginSuccess() {
  await load()
  dialog.success({
    title: '登录成功',
    content: '账号已激活，是否前往「监听源」同步来源？',
    positiveText: '去同步来源',
    negativeText: '稍后',
    onPositiveClick: () => {
      router.push({ name: 'sources' })
    },
  })
}

const columns: DataTableColumns<Account> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '名称', key: 'name' },
  { title: '手机号', key: 'phone_number' },
  { title: 'App ID', key: 'app_id' },
  {
    title: '代理',
    key: 'proxy_id',
    render: (r) => {
      const proxy = proxies.value.find((p) => p.id === r.proxy_id)
      return proxy ? proxy.name : '直连'
    },
  },
  {
    title: '状态',
    key: 'status',
    render: (r) => h(NTag, { type: statusType[r.status] ?? 'default', size: 'small' }, { default: () => r.status }),
  },
  {
    title: '操作',
    key: 'actions',
    render: (r) =>
      h(NSpace, {}, {
        default: () => [
          h(
            NButton,
            { size: 'small', type: 'primary', onClick: () => openLogin(r) },
            { default: () => (r.status === 'active' ? '重新登录' : '登录') },
          ),
          h(NButton, { size: 'small', type: 'error', onClick: () => confirmDelete(r) }, { default: () => '删除' }),
        ],
      }),
  },
]

onMounted(load)
</script>

<template>
  <n-space vertical size="large">
    <PageHeader title="账号" desc="管理 Telegram 登录账号与代理绑定" icon="accounts">
      <template #actions>
        <n-button type="primary" @click="showCreate = true">
          <template #icon><ClayIcon name="plus" :size="16" /></template>
          新建账号
        </n-button>
        <n-button secondary @click="load">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </n-button>
      </template>
    </PageHeader>

    <n-alert type="info" title="登录说明">
      新建账号后点击「登录」，在弹窗中完成 Telegram 登录（发送验证码 → 输入验证码 →
      如有两步验证再输入密码），登录成功后状态变为 active。
    </n-alert>

    <n-data-table
      :loading="loading"
      :columns="columns"
      :data="accounts"
      :bordered="false"
      :scroll-x="820"
    />

    <AccountLoginModal v-model:show="showLogin" :account="loginAccount" @success="onLoginSuccess" />

    <n-modal v-model:show="showCreate" preset="card" title="新建账号" style="width: 520px">
      <n-form label-placement="left" label-width="90">
        <n-form-item label="名称"><n-input v-model:value="form.name" /></n-form-item>
        <n-form-item label="手机号"><n-input v-model:value="form.phone_number" placeholder="+8613800000000" /></n-form-item>
        <n-form-item label="Telegram App">
          <n-select
            v-model:value="form.telegram_app_id"
            :options="appOptions"
            placeholder="选择平台 App 配置"
          />
        </n-form-item>
        <n-form-item label="代理">
          <n-select
            v-model:value="form.proxy_id"
            :options="proxyOptions"
            placeholder="选择代理"
          />
        </n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showCreate = false">取消</n-button>
          <n-button type="primary" @click="create">创建</n-button>
        </n-space>
      </template>
    </n-modal>
  </n-space>
</template>
