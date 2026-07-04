<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import { NButton, useMessage, type DataTableColumns } from 'naive-ui'
import { settingsApi, tokensApi } from '@/api/client'
import { useAuthStore } from '@/stores/auth'
import type { ApiToken, MediaSettingsRequest } from '@/types'
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

// --- 媒体存储设置 ---

const mediaForm = ref({
  dir: 'data/media',
  public_base_url: '',
  url_ttl_hours: 24,
  retention_hours: 168,
  download: {
    image_max_mb: 20,
    file_max_mb: 50,
    file_types: [] as string[],
  },
  s3: {
    enabled: false,
    endpoint: '',
    region: '',
    bucket: '',
    access_key: '',
    use_ssl: true,
    key_prefix: '',
    public_base_url: '',
    auto_cleanup: false,
  },
})
const s3SecretInput = ref('')
const hasS3Secret = ref(false)
const mediaSource = ref<'database' | 'file'>('file')
const mediaLoading = ref(false)
const mediaSaving = ref(false)
const s3Testing = ref(false)

async function loadMedia() {
  mediaLoading.value = true
  try {
    const ms = await settingsApi.media.get()
    mediaForm.value = {
      dir: ms.dir,
      public_base_url: ms.public_base_url,
      url_ttl_hours: ms.url_ttl_hours,
      retention_hours: ms.retention_hours,
      download: {
        image_max_mb: ms.download.image_max_mb,
        file_max_mb: ms.download.file_max_mb,
        file_types: [...(ms.download.file_types ?? [])],
      },
      s3: { ...ms.s3 },
    }
    hasS3Secret.value = ms.has_s3_secret
    mediaSource.value = ms.source
    s3SecretInput.value = ''
  } catch (e) {
    message.error('加载媒体设置失败：' + errText(e))
  } finally {
    mediaLoading.value = false
  }
}

function mediaRequestBody(): MediaSettingsRequest {
  const body: MediaSettingsRequest = {
    ...mediaForm.value,
    download: { ...mediaForm.value.download, file_types: [...mediaForm.value.download.file_types] },
    s3: { ...mediaForm.value.s3 },
  }
  // 留空表示不修改已保存的 secret。
  if (s3SecretInput.value.trim()) {
    body.s3_secret_key = s3SecretInput.value.trim()
  }
  return body
}

async function saveMedia() {
  if (mediaForm.value.s3.enabled) {
    const s3 = mediaForm.value.s3
    if (!s3.endpoint || !s3.bucket || !s3.access_key) {
      message.warning('启用 S3 时 Endpoint、Bucket、Access Key 必填')
      return
    }
    if (!hasS3Secret.value && !s3SecretInput.value.trim()) {
      message.warning('首次启用 S3 需要填写 Secret Key')
      return
    }
  }
  mediaSaving.value = true
  try {
    const ms = await settingsApi.media.update(mediaRequestBody())
    hasS3Secret.value = ms.has_s3_secret
    mediaSource.value = ms.source
    s3SecretInput.value = ''
    message.success('已保存并立即生效，无需重启服务')
  } catch (e) {
    message.error('保存失败：' + errText(e))
  } finally {
    mediaSaving.value = false
  }
}

async function testS3() {
  s3Testing.value = true
  try {
    const res = await settingsApi.media.testS3(mediaRequestBody())
    if (res.success) {
      message.success('S3 连接测试成功')
    } else {
      message.error('S3 连接测试失败：' + (res.error ?? '未知错误'))
    }
  } catch (e) {
    message.error('S3 连接测试失败：' + errText(e))
  } finally {
    s3Testing.value = false
  }
}

onMounted(() => {
  loadTokens()
  loadMedia()
})
</script>

<template>
  <n-space vertical size="large">
    <PageHeader title="设置" desc="管理 API 鉴权与访问凭证" icon="settings" />

    <n-card title="媒体存储">
      <template #header-extra>
        <n-tag size="small" :type="mediaSource === 'database' ? 'success' : 'default'" :bordered="false">
          {{ mediaSource === 'database' ? '页面配置生效中' : '使用配置文件默认值' }}
        </n-tag>
      </template>
      <n-spin :show="mediaLoading">
        <n-space vertical size="large">
          <n-text depth="3">
            Telegram 图片、文件等媒体的下载、存储与公网访问设置。配置「公网访问地址」后，钉钉、Bark、Gotify
            等只认公网 URL 的渠道可以直接收到图片；PDF 等文件的下载还需在监听源上开启「文件下载」开关。
            保存后立即生效，无需重启服务。
          </n-text>

          <n-form label-placement="left" label-width="130" :show-feedback="false">
            <n-space vertical size="medium">
              <n-form-item label="本地媒体目录">
                <n-input v-model:value="mediaForm.dir" placeholder="data/media" />
              </n-form-item>
              <n-form-item label="公网访问地址">
                <n-input
                  v-model:value="mediaForm.public_base_url"
                  placeholder="https://tmf.example.com（本服务对外可访问的根地址，留空则不生成本地媒体 URL）"
                />
              </n-form-item>
              <n-form-item label="URL 有效期(小时)">
                <n-input-number v-model:value="mediaForm.url_ttl_hours" :min="1" style="width: 160px" />
              </n-form-item>
              <n-form-item label="保留时长(小时)">
                <n-space align="center">
                  <n-input-number v-model:value="mediaForm.retention_hours" :min="0" :step="24" style="width: 160px" />
                  <n-text depth="3">超过后自动清理；0 表示不清理，30 天填 720</n-text>
                </n-space>
              </n-form-item>

              <n-divider style="margin: 4px 0">媒体下载策略（Telegram 图片与文件）</n-divider>

              <n-form-item label="图片上限(MB)">
                <n-space align="center">
                  <n-input-number v-model:value="mediaForm.download.image_max_mb" :min="1" style="width: 160px" />
                  <n-text depth="3">超过上限的图片不下载，按文本摘要降级</n-text>
                </n-space>
              </n-form-item>
              <n-form-item label="文件上限(MB)">
                <n-space align="center">
                  <n-input-number v-model:value="mediaForm.download.file_max_mb" :min="1" style="width: 160px" />
                  <n-text depth="3">PDF 等文件的下载上限；文件下载需在监听源上单独开启</n-text>
                </n-space>
              </n-form-item>
              <n-form-item label="文件类型白名单">
                <n-space vertical size="small" style="width: 100%">
                  <n-dynamic-tags v-model:value="mediaForm.download.file_types" />
                  <n-text depth="3">按扩展名过滤（不带点，如 pdf、docx、zip）；清空表示不限类型，仅按大小限制</n-text>
                </n-space>
              </n-form-item>

              <n-divider style="margin: 4px 0">
                S3 兼容对象存储（AWS S3 / Cloudflare R2 / MinIO / OSS / COS，可选）
              </n-divider>

              <n-form-item label="启用 S3">
                <n-switch v-model:value="mediaForm.s3.enabled" />
              </n-form-item>
              <template v-if="mediaForm.s3.enabled">
                <n-form-item label="Endpoint">
                  <n-input v-model:value="mediaForm.s3.endpoint" placeholder="如 xxx.r2.cloudflarestorage.com（不带协议）" />
                </n-form-item>
                <n-form-item label="Region">
                  <n-input v-model:value="mediaForm.s3.region" placeholder="可选，如 us-east-1；R2 填 auto" />
                </n-form-item>
                <n-form-item label="Bucket">
                  <n-input v-model:value="mediaForm.s3.bucket" placeholder="桶名" />
                </n-form-item>
                <n-form-item label="Access Key">
                  <n-input v-model:value="mediaForm.s3.access_key" placeholder="访问密钥 ID" />
                </n-form-item>
                <n-form-item label="Secret Key">
                  <n-input
                    v-model:value="s3SecretInput"
                    type="password"
                    show-password-on="click"
                    :placeholder="hasS3Secret ? '已配置，留空表示不修改' : '访问密钥 Secret'"
                  />
                </n-form-item>
                <n-form-item label="使用 HTTPS">
                  <n-switch v-model:value="mediaForm.s3.use_ssl" />
                </n-form-item>
                <n-form-item label="Key 前缀">
                  <n-input v-model:value="mediaForm.s3.key_prefix" placeholder="可选；与其他数据共用桶时建议设置，如 tmf" />
                </n-form-item>
                <n-form-item label="公开访问地址">
                  <n-input
                    v-model:value="mediaForm.s3.public_base_url"
                    placeholder="可选；公开桶或 CDN 根地址，留空则生成预签名 URL"
                  />
                </n-form-item>
                <n-form-item label="自动清理对象">
                  <n-space align="center">
                    <n-switch v-model:value="mediaForm.s3.auto_cleanup" />
                    <n-text depth="3">按上方保留时长删除过期对象；共用桶时务必设置 Key 前缀</n-text>
                  </n-space>
                </n-form-item>
              </template>
            </n-space>
          </n-form>

          <n-space>
            <n-button type="primary" :loading="mediaSaving" @click="saveMedia">保存媒体设置</n-button>
            <n-button v-if="mediaForm.s3.enabled" secondary :loading="s3Testing" @click="testS3">
              测试 S3 连接
            </n-button>
          </n-space>
        </n-space>
      </n-spin>
    </n-card>

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
