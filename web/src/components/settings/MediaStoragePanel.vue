<script setup lang="ts">
import { computed, onMounted, ref, shallowRef } from 'vue'
import { useMessage } from 'naive-ui'
import { settingsApi } from '@/api/client'
import type { MediaSettings, MediaSettingsRequest } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const defaultForm: MediaSettingsRequest = {
  dir: 'data/media',
  public_base_url: '',
  url_ttl_hours: 24,
  retention_hours: 168,
  download: { image_max_mb: 20, file_max_mb: 50, file_types: [] },
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
}

const form = ref<MediaSettingsRequest>(structuredClone(defaultForm))
const saved = ref<MediaSettingsRequest>(structuredClone(defaultForm))
const s3SecretInput = shallowRef('')
const hasS3Secret = shallowRef(false)
const source = shallowRef<'database' | 'file'>('file')
const loading = shallowRef(false)
const loaded = shallowRef(false)
const saving = shallowRef(false)
const s3Testing = shallowRef(false)
const cleaning = shallowRef(false)
const showCleanup = shallowRef(false)
const deleteLocal = shallowRef(true)
const deleteRemote = shallowRef(false)

const isDirty = computed(
  () => JSON.stringify(form.value) !== JSON.stringify(saved.value) || s3SecretInput.value.trim() !== '',
)
const canCleanup = computed(() => loaded.value && saved.value.retention_hours > 0)

function toRequest(settings: MediaSettings): MediaSettingsRequest {
  return {
    dir: settings.dir,
    public_base_url: settings.public_base_url,
    url_ttl_hours: settings.url_ttl_hours,
    retention_hours: settings.retention_hours,
    download: {
      image_max_mb: settings.download.image_max_mb,
      file_max_mb: settings.download.file_max_mb,
      file_types: [...(settings.download.file_types ?? [])],
    },
    s3: { ...settings.s3 },
  }
}

function assignSettings(settings: MediaSettings): void {
  const value = toRequest(settings)
  form.value = structuredClone(value)
  saved.value = structuredClone(value)
  hasS3Secret.value = settings.has_s3_secret
  source.value = settings.source
  s3SecretInput.value = ''
}

async function load(): Promise<void> {
  loading.value = true
  loaded.value = false
  try {
    assignSettings(await settingsApi.media.get())
    loaded.value = true
  } catch (error) {
    message.error('加载媒体设置失败：' + errText(error))
  } finally {
    loading.value = false
  }
}

function requestBody(): MediaSettingsRequest {
  const body = structuredClone(form.value)
  if (s3SecretInput.value.trim()) body.s3_secret_key = s3SecretInput.value.trim()
  return body
}

async function save(): Promise<void> {
  if (form.value.s3.enabled) {
    const s3 = form.value.s3
    if (!s3.endpoint || !s3.bucket || !s3.access_key) {
      message.warning('启用 S3 时 Endpoint、Bucket、Access Key 必填')
      return
    }
    if (!hasS3Secret.value && !s3SecretInput.value.trim()) {
      message.warning('首次启用 S3 需要填写 Secret Key')
      return
    }
  }
  saving.value = true
  try {
    assignSettings(await settingsApi.media.update(requestBody()))
    message.success('已保存并立即生效，无需重启服务')
  } catch (error) {
    message.error('保存失败：' + errText(error))
  } finally {
    saving.value = false
  }
}

async function testS3(): Promise<void> {
  s3Testing.value = true
  try {
    const result = await settingsApi.media.testS3(requestBody())
    if (result.success) message.success('S3 连接测试成功')
    else message.error('S3 连接测试失败：' + (result.error ?? '未知错误'))
  } catch (error) {
    message.error('S3 连接测试失败：' + errText(error))
  } finally {
    s3Testing.value = false
  }
}

function openCleanup(): void {
  deleteLocal.value = true
  deleteRemote.value = false
  showCleanup.value = true
}

async function cleanup(): Promise<void> {
  if (!deleteLocal.value && !deleteRemote.value) {
    message.warning('至少选择一个清理目标')
    return
  }
  cleaning.value = true
  try {
    const result = await settingsApi.media.cleanup({
      delete_local: deleteLocal.value,
      delete_remote: deleteRemote.value,
    })
    showCleanup.value = false
    message.success(`已清理本地文件 ${result.deleted_local_files} 个、远端对象 ${result.deleted_remote_objects} 个`)
  } catch (error) {
    message.error('清理媒体失败：' + errText(error))
  } finally {
    cleaning.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <n-card title="媒体存储">
    <template #header-extra>
      <n-tag size="small" :type="source === 'database' ? 'success' : 'default'" :bordered="false">
        {{ source === 'database' ? '页面配置生效中' : '使用配置文件默认值' }}
      </n-tag>
    </template>
    <n-spin :show="loading">
      <n-space vertical size="large">
        <n-text depth="3">
          Telegram 图片、文件等媒体的下载、存储与公网访问设置。保存后立即生效，无需重启服务。
        </n-text>
        <n-form label-placement="left" label-width="130" :show-feedback="false">
          <n-space vertical size="medium">
            <n-form-item label="本地媒体目录"><n-input v-model:value="form.dir" placeholder="data/media" /></n-form-item>
            <n-form-item label="公网访问地址">
              <n-input v-model:value="form.public_base_url" placeholder="https://tmf.example.com" />
            </n-form-item>
            <n-form-item label="URL 有效期(小时)">
              <n-input-number v-model:value="form.url_ttl_hours" :min="1" class="number-input" />
            </n-form-item>
            <n-form-item label="保留时长(小时)">
              <n-space align="center">
                <n-input-number v-model:value="form.retention_hours" :min="0" :step="24" class="number-input" />
                <n-text depth="3">超过后自动清理；0 表示不清理，30 天填 720</n-text>
              </n-space>
            </n-form-item>

            <n-divider class="section-divider">媒体下载策略（Telegram 图片与文件）</n-divider>
            <n-form-item label="图片上限(MB)">
              <n-space align="center">
                <n-input-number v-model:value="form.download.image_max_mb" :min="1" class="number-input" />
                <n-text depth="3">超过上限的图片不下载，按文本摘要降级</n-text>
              </n-space>
            </n-form-item>
            <n-form-item label="文件上限(MB)">
              <n-space align="center">
                <n-input-number v-model:value="form.download.file_max_mb" :min="1" class="number-input" />
                <n-text depth="3">文件下载需在监听源上单独开启</n-text>
              </n-space>
            </n-form-item>
            <n-form-item label="文件类型白名单">
              <n-space vertical size="small" class="full-width">
                <n-dynamic-tags v-model:value="form.download.file_types" />
                <n-text depth="3">按扩展名过滤；清空表示不限类型，仅按大小限制</n-text>
              </n-space>
            </n-form-item>

            <n-divider class="section-divider">S3 兼容对象存储（可选）</n-divider>
            <n-form-item label="启用 S3"><n-switch v-model:value="form.s3.enabled" /></n-form-item>
            <template v-if="form.s3.enabled">
              <n-form-item label="Endpoint"><n-input v-model:value="form.s3.endpoint" /></n-form-item>
              <n-form-item label="Region"><n-input v-model:value="form.s3.region" placeholder="如 us-east-1；R2 填 auto" /></n-form-item>
              <n-form-item label="Bucket"><n-input v-model:value="form.s3.bucket" /></n-form-item>
              <n-form-item label="Access Key"><n-input v-model:value="form.s3.access_key" /></n-form-item>
              <n-form-item label="Secret Key">
                <n-input
                  v-model:value="s3SecretInput"
                  type="password"
                  show-password-on="click"
                  :placeholder="hasS3Secret ? '已配置，留空表示不修改' : '访问密钥 Secret'"
                />
              </n-form-item>
              <n-form-item label="使用 HTTPS"><n-switch v-model:value="form.s3.use_ssl" /></n-form-item>
              <n-form-item label="Key 前缀">
                <n-input v-model:value="form.s3.key_prefix" placeholder="共用桶时建议设置，如 tmf" />
              </n-form-item>
              <n-form-item label="公开访问地址"><n-input v-model:value="form.s3.public_base_url" /></n-form-item>
              <n-form-item label="自动清理对象">
                <n-space align="center">
                  <n-switch v-model:value="form.s3.auto_cleanup" />
                  <n-text depth="3">按保留时长删除过期对象；共用桶时务必设置 Key 前缀</n-text>
                </n-space>
              </n-form-item>
            </template>
          </n-space>
        </n-form>

        <n-space>
          <n-button type="primary" :loading="saving" @click="save">保存媒体设置</n-button>
          <n-button v-if="form.s3.enabled" secondary :loading="s3Testing" @click="testS3">测试 S3 连接</n-button>
          <n-button
            secondary
            type="error"
            :disabled="isDirty || !canCleanup"
            :title="isDirty ? '请先保存当前修改' : !canCleanup ? '媒体保留时长为 0' : undefined"
            @click="openCleanup"
          >
            立即清理过期媒体
          </n-button>
        </n-space>
      </n-space>
    </n-spin>

    <n-modal v-model:show="showCleanup" preset="dialog" title="立即清理过期媒体">
      <n-space vertical size="large">
        <n-alert type="warning" :show-icon="true">
          清理不可撤销，将删除修改时间早于 {{ saved.retention_hours }} 小时的媒体。
        </n-alert>
        <n-checkbox v-model:checked="deleteLocal">本地缓存：{{ saved.dir }}</n-checkbox>
        <n-checkbox v-model:checked="deleteRemote" :disabled="!saved.s3.enabled">
          S3 对象：{{ saved.s3.enabled ? `${saved.s3.bucket}/${saved.s3.key_prefix}` : '未启用' }}
        </n-checkbox>
        <n-alert v-if="deleteRemote && !saved.s3.key_prefix" type="error" :show-icon="true">
          Key 前缀为空，本次操作会扫描并清理整个 Bucket 中的过期对象。
        </n-alert>
      </n-space>
      <template #action>
        <n-space justify="end">
          <n-button :disabled="cleaning" @click="showCleanup = false">取消</n-button>
          <n-button type="error" :loading="cleaning" :disabled="!deleteLocal && !deleteRemote" @click="cleanup">
            确认清理
          </n-button>
        </n-space>
      </template>
    </n-modal>
  </n-card>
</template>

<style scoped>
.number-input {
  width: 160px;
}

.full-width {
  width: 100%;
}

.section-divider {
  margin: 4px 0;
}
</style>
