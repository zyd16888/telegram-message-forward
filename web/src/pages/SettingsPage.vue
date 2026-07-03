<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import { NButton, useMessage, type DataTableColumns } from 'naive-ui'
import { tokensApi } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import type { ApiToken } from '@/types'
import { errText } from '@/utils/error'
import PageHeader from '@/components/PageHeader.vue'

const message = useMessage()
const auth = useAuthStore()

const tokens = ref<ApiToken[]>([])
const newTokenName = ref('')
const createdToken = ref('')
const loading = ref(false)

// 当前鉴权模式描述。
const authStatus = computed(() =>
  auth.devNoAuth
    ? { type: 'warning' as const, title: '开发免鉴权', desc: '当前后端已关闭管理 API 鉴权（auth_enabled=false）。Token 管理为可选。' }
    : { type: 'success' as const, title: '鉴权已启用', desc: '管理 API 需要有效 Bearer Token。登录入口在登录页，Settings 仅用于凭证维护。' },
)

async function loadTokens() {
  loading.value = true
  try {
    tokens.value = await tokensApi.list()
  } catch (e) {
    message.error('加载 token 列表失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

async function createToken() {
  if (!newTokenName.value.trim()) {
    message.warning('请填写 token 名称')
    return
  }
  try {
    const res = await tokensApi.create(newTokenName.value.trim())
    createdToken.value = res.token
    newTokenName.value = ''
    message.success('已创建，请立即保存明文 token')
    await loadTokens()
  } catch (e) {
    message.error('创建失败：' + errText(e))
  }
}

async function revoke(id: number) {
  try {
    await tokensApi.revoke(id)
    message.success('已吊销')
    await loadTokens()
  } catch (e) {
    message.error('吊销失败：' + errText(e))
  }
}

const columns: DataTableColumns<ApiToken> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '名称', key: 'name' },
  { title: '状态', key: 'revoked', render: (r) => (r.revoked ? '已吊销' : '有效') },
  { title: '创建时间', key: 'created_at' },
  {
    title: '操作',
    key: 'actions',
    render: (r) =>
      h(
        NButton,
        { size: 'small', type: 'error', disabled: r.revoked, onClick: () => revoke(r.id) },
        { default: () => '吊销' },
      ),
  },
]

onMounted(loadTokens)
</script>

<template>
  <n-space vertical size="large">
    <PageHeader title="设置" desc="管理 API 鉴权与访问凭证" icon="settings" />

    <n-card title="API 鉴权状态">
      <n-alert :type="authStatus.type" :title="authStatus.title">
        {{ authStatus.desc }}
      </n-alert>
    </n-card>

    <n-card title="Token 管理（管理凭证维护）">
      <n-space vertical>
        <n-text depth="3">
          在此维护管理凭证：生成新凭证、吊销旧凭证。生成的明文 Token 仅显示一次。
        </n-text>
        <n-input-group>
          <n-input v-model:value="newTokenName" placeholder="新 token 名称" />
          <n-button type="primary" @click="createToken">生成新 Token</n-button>
        </n-input-group>
        <n-alert
          v-if="createdToken"
          type="success"
          title="明文 Token（仅显示一次）"
          closable
          @close="createdToken = ''"
        >
          <n-text code>{{ createdToken }}</n-text>
        </n-alert>

        <n-data-table :loading="loading" :columns="columns" :data="tokens" :bordered="false" :scroll-x="560" />
      </n-space>
    </n-card>
  </n-space>
</template>
