<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { NButton, NSpace, NTag, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { telegramConfigApi } from '@/api/client'
import type { SharedProxy, TelegramApp } from '@/types'
import { errText } from '@/utils/error'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'

const message = useMessage()
const dialog = useDialog()

const apps = ref<TelegramApp[]>([])
const proxies = ref<SharedProxy[]>([])
const loading = ref(false)
const showAppModal = ref(false)
const showProxyModal = ref(false)
const editingApp = ref<TelegramApp | null>(null)
const editingProxy = ref<SharedProxy | null>(null)

const appForm = ref({
  name: '',
  app_id: null as number | null,
  app_hash: '',
  enabled: true,
})

const proxyForm = ref({
  name: '',
  type: 'socks5',
  addr: '',
  username: '',
  password: '',
  enabled: true,
})

async function load() {
  loading.value = true
  try {
    apps.value = await telegramConfigApi.apps.list()
    proxies.value = await telegramConfigApi.proxies.list()
  } catch (e) {
    message.error('加载失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

function openCreateApp() {
  editingApp.value = null
  appForm.value = { name: '', app_id: null, app_hash: '', enabled: true }
  showAppModal.value = true
}

function openEditApp(row: TelegramApp) {
  editingApp.value = row
  appForm.value = { name: row.name, app_id: row.app_id, app_hash: '', enabled: row.enabled }
  showAppModal.value = true
}

async function saveApp() {
  if (!appForm.value.app_id) {
    message.warning('请填写 App ID')
    return
  }
  const body: Record<string, unknown> = {
    name: appForm.value.name,
    app_id: appForm.value.app_id,
    enabled: appForm.value.enabled,
  }
  if (appForm.value.app_hash) body.app_hash = appForm.value.app_hash
  try {
    if (editingApp.value) {
      await telegramConfigApi.apps.update(editingApp.value.id, body)
      message.success('Telegram App 已更新')
    } else {
      if (!appForm.value.app_hash) {
        message.warning('新建 App 需要填写 App Hash')
        return
      }
      await telegramConfigApi.apps.create(body)
      message.success('Telegram App 已创建')
    }
    showAppModal.value = false
    await load()
  } catch (e) {
    message.error('保存失败：' + errText(e))
  }
}

function confirmDeleteApp(row: TelegramApp) {
  dialog.warning({
    title: '删除 Telegram App',
    content: `确定删除「${row.name}」？已被账号引用时数据库会拒绝删除。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await telegramConfigApi.apps.remove(row.id)
        message.success('已删除')
        await load()
      } catch (e) {
        message.error('删除失败：' + errText(e))
      }
    },
  })
}

function openCreateProxy() {
  editingProxy.value = null
  proxyForm.value = { name: '', type: 'socks5', addr: '', username: '', password: '', enabled: true }
  showProxyModal.value = true
}

function openEditProxy(row: SharedProxy) {
  editingProxy.value = row
  proxyForm.value = {
    name: row.name,
    type: row.type,
    addr: row.addr,
    username: row.username ?? '',
    password: '',
    enabled: row.enabled,
  }
  showProxyModal.value = true
}

async function saveProxy() {
  const body: Record<string, unknown> = {
    name: proxyForm.value.name,
    type: proxyForm.value.type,
    addr: proxyForm.value.addr,
    username: proxyForm.value.username,
    enabled: proxyForm.value.enabled,
  }
  if (proxyForm.value.password) body.password = proxyForm.value.password
  try {
    if (editingProxy.value) {
      await telegramConfigApi.proxies.update(editingProxy.value.id, body)
      message.success('代理已更新')
    } else {
      await telegramConfigApi.proxies.create(body)
      message.success('代理已创建')
    }
    showProxyModal.value = false
    await load()
  } catch (e) {
    message.error('保存失败：' + errText(e))
  }
}

function confirmDeleteProxy(row: SharedProxy) {
  dialog.warning({
    title: '删除代理',
    content: `确定删除「${row.name}」？使用中的账号会自动变为直连。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await telegramConfigApi.proxies.remove(row.id)
        message.success('已删除')
        await load()
      } catch (e) {
        message.error('删除失败：' + errText(e))
      }
    },
  })
}

const appColumns: DataTableColumns<TelegramApp> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '名称', key: 'name' },
  { title: 'App ID', key: 'app_id' },
  { title: 'App Hash', key: 'has_hash', render: (r) => (r.has_hash ? '已配置' : '未配置') },
  {
    title: '状态',
    key: 'enabled',
    render: (r) => h(NTag, { type: r.enabled ? 'success' : 'default', size: 'small' }, { default: () => (r.enabled ? '启用' : '停用') }),
  },
  {
    title: '操作',
    key: 'actions',
    render: (r) =>
      h(NSpace, {}, {
        default: () => [
          h(NButton, { size: 'small', onClick: () => openEditApp(r) }, { default: () => '编辑' }),
          h(NButton, { size: 'small', type: 'error', onClick: () => confirmDeleteApp(r) }, { default: () => '删除' }),
        ],
      }),
  },
]

const proxyColumns: DataTableColumns<SharedProxy> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '名称', key: 'name' },
  { title: '类型', key: 'type' },
  { title: '地址', key: 'addr' },
  { title: '用户名', key: 'username' },
  { title: '密码', key: 'has_password', render: (r) => (r.has_password ? '已配置' : '未配置') },
  {
    title: '状态',
    key: 'enabled',
    render: (r) => h(NTag, { type: r.enabled ? 'success' : 'default', size: 'small' }, { default: () => (r.enabled ? '启用' : '停用') }),
  },
  {
    title: '操作',
    key: 'actions',
    render: (r) =>
      h(NSpace, {}, {
        default: () => [
          h(NButton, { size: 'small', onClick: () => openEditProxy(r) }, { default: () => '编辑' }),
          h(NButton, { size: 'small', type: 'error', onClick: () => confirmDeleteProxy(r) }, { default: () => '删除' }),
        ],
      }),
  },
]

onMounted(load)
</script>

<template>
  <n-space vertical size="large">
    <PageHeader title="Telegram 配置" desc="维护 Telegram App 凭证与共享代理" icon="telegram" />

    <n-card title="Telegram App">
      <template #header-extra>
        <n-button type="primary" @click="openCreateApp">
          <template #icon><ClayIcon name="plus" :size="16" /></template>
          新增 App
        </n-button>
      </template>
      <n-data-table :loading="loading" :columns="appColumns" :data="apps" :bordered="false" :scroll-x="640" />
    </n-card>

    <n-card title="代理">
      <template #header-extra>
        <n-button type="primary" @click="openCreateProxy">
          <template #icon><ClayIcon name="plus" :size="16" /></template>
          新增代理
        </n-button>
      </template>
      <n-data-table :loading="loading" :columns="proxyColumns" :data="proxies" :bordered="false" :scroll-x="720" />
    </n-card>

    <n-modal v-model:show="showAppModal" preset="card" title="Telegram App" style="width: 520px">
      <n-form label-placement="left" label-width="90">
        <n-form-item label="名称"><n-input v-model:value="appForm.name" /></n-form-item>
        <n-form-item label="App ID"><n-input-number v-model:value="appForm.app_id" :show-button="false" style="width: 100%" /></n-form-item>
        <n-form-item label="App Hash">
          <n-input v-model:value="appForm.app_hash" type="password" show-password-on="click" placeholder="编辑时留空表示不修改" />
        </n-form-item>
        <n-form-item label="启用"><n-switch v-model:value="appForm.enabled" /></n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showAppModal = false">取消</n-button>
          <n-button type="primary" @click="saveApp">保存</n-button>
        </n-space>
      </template>
    </n-modal>

    <n-modal v-model:show="showProxyModal" preset="card" title="代理" style="width: 520px">
      <n-form label-placement="left" label-width="90">
        <n-form-item label="名称"><n-input v-model:value="proxyForm.name" /></n-form-item>
        <n-form-item label="类型">
          <n-select
            v-model:value="proxyForm.type"
            :options="[
              { label: 'SOCKS5', value: 'socks5' },
              { label: 'HTTP', value: 'http' },
              { label: 'HTTPS', value: 'https' },
            ]"
          />
        </n-form-item>
        <n-form-item label="地址"><n-input v-model:value="proxyForm.addr" placeholder="host:port" /></n-form-item>
        <n-form-item label="用户名"><n-input v-model:value="proxyForm.username" /></n-form-item>
        <n-form-item label="密码">
          <n-input v-model:value="proxyForm.password" type="password" show-password-on="click" placeholder="编辑时留空表示不修改" />
        </n-form-item>
        <n-form-item label="启用"><n-switch v-model:value="proxyForm.enabled" /></n-form-item>
      </n-form>
      <template #footer>
        <n-space justify="end">
          <n-button @click="showProxyModal = false">取消</n-button>
          <n-button type="primary" @click="saveProxy">保存</n-button>
        </n-space>
      </template>
    </n-modal>
  </n-space>
</template>
