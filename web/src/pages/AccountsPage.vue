<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NSpace, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { accountsApi } from '@/api/client'
import type { Account } from '@/types'
import { errText } from '@/utils/error'
import AccountLoginModal from '@/components/AccountLoginModal.vue'

const message = useMessage()
const dialog = useDialog()
const router = useRouter()

const accounts = ref<Account[]>([])
const loading = ref(false)
const showCreate = ref(false)
const showLogin = ref(false)
const loginAccount = ref<Account | null>(null)

const form = ref({
  name: '',
  phone_number: '',
  app_id: 0,
  app_hash: '',
  proxy: { type: '', addr: '', username: '', password: '' },
})

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
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

async function create() {
  try {
    const body: Record<string, unknown> = {
      name: form.value.name,
      phone_number: form.value.phone_number,
      app_id: Number(form.value.app_id),
      app_hash: form.value.app_hash,
    }
    if (form.value.proxy.addr) body.proxy = form.value.proxy
    const created = await accountsApi.create(body)
    message.success('账号已创建，点击「登录」完成 Telegram 登录')
    showCreate.value = false
    // 新建后自动打开登录弹窗。
    openLogin(created)
    form.value = { name: '', phone_number: '', app_id: 0, app_hash: '', proxy: { type: '', addr: '', username: '', password: '' } }
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
    <n-space justify="space-between">
      <n-button type="primary" @click="showCreate = true">新建账号</n-button>
      <n-button @click="load">刷新</n-button>
    </n-space>

    <n-alert type="info" title="登录说明">
      新建账号后点击「登录」，在弹窗中完成 Telegram 登录（发送验证码 → 输入验证码 →
      如有两步验证再输入密码），登录成功后状态变为 active。
    </n-alert>

    <n-data-table :loading="loading" :columns="columns" :data="accounts" :bordered="false" />

    <AccountLoginModal v-model:show="showLogin" :account="loginAccount" @success="onLoginSuccess" />

    <n-modal v-model:show="showCreate" preset="card" title="新建账号" style="width: 520px">
      <n-form label-placement="left" label-width="90">
        <n-form-item label="名称"><n-input v-model:value="form.name" /></n-form-item>
        <n-form-item label="手机号"><n-input v-model:value="form.phone_number" placeholder="+8613800000000" /></n-form-item>
        <n-form-item label="App ID"><n-input-number v-model:value="form.app_id" :show-button="false" style="width: 100%" /></n-form-item>
        <n-form-item label="App Hash"><n-input v-model:value="form.app_hash" type="password" show-password-on="click" /></n-form-item>
        <n-divider>代理（可选）</n-divider>
        <n-form-item label="类型">
          <n-select
            v-model:value="form.proxy.type"
            :options="[{ label: '无', value: '' }, { label: 'socks5', value: 'socks5' }]"
          />
        </n-form-item>
        <n-form-item label="地址"><n-input v-model:value="form.proxy.addr" placeholder="host:port" /></n-form-item>
        <n-form-item label="用户名"><n-input v-model:value="form.proxy.username" /></n-form-item>
        <n-form-item label="密码"><n-input v-model:value="form.proxy.password" type="password" show-password-on="click" /></n-form-item>
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
