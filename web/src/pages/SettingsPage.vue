<script setup lang="ts">
import { h, onMounted, ref } from 'vue'
import { NButton, useMessage, type DataTableColumns } from 'naive-ui'
import { tokensApi } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import type { ApiToken } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const auth = useAuthStore()

const tokenInput = ref(auth.token)
const tokens = ref<ApiToken[]>([])
const newTokenName = ref('')
const createdToken = ref('')
const loading = ref(false)

function saveToken() {
  auth.save(tokenInput.value.trim())
  message.success('Token 已保存到本地')
  void loadTokens()
}

async function loadTokens() {
  if (!auth.hasToken) return
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
    <n-card title="API Token（本地）">
      <n-space vertical>
        <n-text depth="3">
          管理 API 需要 Bearer Token。首个 token 用
          <n-text code>go run ./cmd/token create</n-text> 生成，粘贴到此处保存。
        </n-text>
        <n-input-group>
          <n-input
            v-model:value="tokenInput"
            type="password"
            show-password-on="click"
            placeholder="粘贴 API token"
          />
          <n-button type="primary" @click="saveToken">保存</n-button>
        </n-input-group>
      </n-space>
    </n-card>

    <n-card title="Token 管理">
      <n-space vertical>
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

        <n-data-table :loading="loading" :columns="columns" :data="tokens" :bordered="false" />
      </n-space>
    </n-card>
  </n-space>
</template>
