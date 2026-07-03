<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useMessage } from 'naive-ui'
import ConfigFormRenderer from '@/components/ConfigFormRenderer.vue'
import { sinksApi } from '@/api/client'
import type { Capabilities, MediaCapability, Sink, SinkDescriptor } from '@/types'
import { errText } from '@/utils/error'

const show = defineModel<boolean>('show', { required: true })

const props = defineProps<{
  descriptors: SinkDescriptor[]
  sink?: Sink | null
}>()

const emit = defineEmits<{
  saved: []
}>()

const message = useMessage()

const form = reactive({
  type: '',
  name: '',
  enabled: true,
  config: {} as Record<string, unknown>,
  secret: '',
})
const testing = reactive({
  loading: false,
  success: null as boolean | null,
  message: '',
  summary: '',
})

const editing = computed(() => Boolean(props.sink))
const descriptor = computed(() => props.descriptors.find((item) => item.type === form.type) ?? props.descriptors[0])
const typeOptions = computed(() => props.descriptors.map((item) => ({ label: item.label, value: item.type })))
const capabilities = computed(() => descriptor.value?.capabilities)
const formatCapabilities = computed(() => formatItems(capabilities.value))
const mediaCapabilities = computed(() => capabilities.value?.media ?? [])

watch(
  () => [show.value, props.sink, props.descriptors] as const,
  () => {
    if (!show.value) return
    resetForm()
  },
  { immediate: true },
)

watch(
  () => form.type,
  (type, oldType) => {
    if (!type || type === oldType || editing.value) return
    form.config = defaultsFor(descriptor.value)
    form.secret = ''
  },
)

function resetForm() {
  if (props.sink) {
    form.type = props.sink.type
    form.name = props.sink.name
    form.enabled = props.sink.enabled
    form.config = { ...props.sink.config }
    form.secret = ''
    return
  }
  const first = props.descriptors[0]
  form.type = first?.type ?? ''
  form.name = ''
  form.enabled = true
  form.config = defaultsFor(first)
  form.secret = ''
  resetTestResult()
}

function defaultsFor(desc?: SinkDescriptor): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const field of desc?.config_fields ?? []) {
    if (field.default !== undefined) out[field.key] = field.default
  }
  return out
}

function formatItems(caps?: Capabilities) {
  return [
    { key: 'text', label: 'Text', supported: Boolean(caps?.supports_text) },
    { key: 'markdown', label: 'Markdown', supported: Boolean(caps?.supports_markdown) },
    { key: 'html', label: 'HTML', supported: Boolean(caps?.supports_html) },
  ]
}

function mediaTypeLabel(type: string): string {
  const labels: Record<string, string> = {
    image: '图片',
    file: '文件',
    audio: '音频',
    video: '视频',
  }
  return labels[type] ?? type
}

function mediaDetail(item: MediaCapability): string {
  if (!item.supported) return item.fallback || '不支持时按文本降级'
  const parts: string[] = []
  if (item.max_size_mb) parts.push(`<= ${item.max_size_mb} MB`)
  if (item.requires_upload) parts.push('需上传')
  if (item.supports_public_url) parts.push('支持公网 URL')
  if (item.supports_binary) parts.push('支持二进制')
  if (item.delivery_mode) parts.push(item.delivery_mode)
  return parts.join(' / ') || '支持'
}

function validate(): boolean {
  if (!form.type) {
    message.warning('请选择渠道类型')
    return false
  }
  if (!form.name.trim()) {
    message.warning('请填写渠道名称')
    return false
  }
  const secretField = descriptor.value?.secret_field
  if (!editing.value && secretField?.required && !form.secret.trim()) {
    message.warning(`请填写${secretField.label}`)
    return false
  }
  return true
}

function resetTestResult() {
  testing.loading = false
  testing.success = null
  testing.message = ''
  testing.summary = ''
}

function testPayload(): Record<string, unknown> {
  const body: Record<string, unknown> = {
    type: form.type,
    config: form.config,
  }
  if (form.secret) body.secret = form.secret
  return body
}

async function testConfig() {
  if (!validate()) return
  testing.loading = true
  testing.success = null
  testing.message = ''
  testing.summary = ''
  try {
    const result = props.sink
      ? await sinksApi.testExisting(props.sink.id, testPayload())
      : await sinksApi.test(testPayload())
    testing.success = result.success
    testing.message = result.success ? '测试消息已发送成功' : result.error || '测试失败'
    testing.summary = result.response_summary ? JSON.stringify(result.response_summary, null, 2) : ''
    if (props.sink) emit('saved')
  } catch (e) {
    testing.success = false
    testing.message = errText(e)
    if (props.sink) emit('saved')
  } finally {
    testing.loading = false
  }
}

async function submit() {
  if (!validate()) return
  try {
    if (props.sink) {
      const body: Record<string, unknown> = {
        name: form.name,
        enabled: form.enabled,
        config: form.config,
      }
      if (form.secret) body.secret = form.secret
      await sinksApi.update(props.sink.id, body)
    } else {
      await sinksApi.create({
        type: form.type,
        name: form.name,
        enabled: form.enabled,
        config: form.config,
        secret: form.secret,
      })
    }
    message.success('已保存渠道')
    show.value = false
    emit('saved')
  } catch (e) {
    message.error('保存失败：' + errText(e))
  }
}
</script>

<template>
  <NModal
    v-model:show="show"
    preset="card"
    :title="editing ? '编辑渠道' : '新建渠道'"
    class="sink-modal"
    :style="{ width: 'min(600px, calc(100vw - 32px))' }"
  >
    <div class="modal-body">
      <NForm label-placement="top">
        <section class="form-section">
          <div class="section-title">基础信息</div>
          <NFormItem label="类型" required>
            <NSelect v-model:value="form.type" :disabled="editing" :options="typeOptions" />
          </NFormItem>
          <NFormItem label="名称" required>
            <NInput v-model:value="form.name" placeholder="例如：技术群通知" />
          </NFormItem>
          <NFormItem label="启用">
            <NSwitch v-model:value="form.enabled" />
          </NFormItem>
        </section>

        <section class="form-section">
          <div class="section-title">渠道配置</div>
          <NAlert v-if="descriptor?.description" type="default" :show-icon="false" class="sink-desc">
            {{ descriptor.description }}
          </NAlert>
          <div v-if="capabilities" class="capability-panel">
            <div class="cap-row">
              <span class="cap-label">格式</span>
              <NSpace size="small">
                <NTag
                  v-for="item in formatCapabilities"
                  :key="item.key"
                  size="small"
                  :type="item.supported ? 'success' : 'default'"
                  :bordered="false"
                >
                  {{ item.label }}
                </NTag>
              </NSpace>
            </div>
            <div v-if="capabilities.max_text_length" class="cap-row">
              <span class="cap-label">文本上限</span>
              <NText>{{ capabilities.max_text_length }} 字符/字节，按渠道官方口径执行</NText>
            </div>
            <div v-if="mediaCapabilities.length" class="cap-row cap-row-media">
              <span class="cap-label">媒体</span>
              <div class="media-caps">
                <div v-for="item in mediaCapabilities" :key="item.type" class="media-cap-item">
                  <NTag size="small" :type="item.supported ? 'success' : 'warning'" :bordered="false">
                    {{ mediaTypeLabel(item.type) }}{{ item.supported ? '支持' : '降级' }}
                  </NTag>
                  <NText depth="3">{{ mediaDetail(item) }}</NText>
                </div>
              </div>
            </div>
            <NText v-for="note in capabilities.notes ?? []" :key="note" class="cap-note" depth="3">
              {{ note }}
            </NText>
          </div>
          <ConfigFormRenderer v-if="descriptor" v-model="form.config" :fields="descriptor.config_fields" />
          <NFormItem v-if="descriptor?.secret_field" :label="descriptor.secret_field.label" :required="!editing && descriptor.secret_field.required">
            <NInput
              v-model:value="form.secret"
              type="password"
              show-password-on="click"
              :placeholder="editing ? '留空则保留原密钥' : descriptor.secret_field.placeholder"
            />
          </NFormItem>
        </section>

        <NAlert
          v-if="testing.success !== null"
          class="test-result"
          :type="testing.success ? 'success' : 'error'"
          :title="testing.success ? '测试成功' : '测试失败'"
        >
          <NSpace vertical size="small">
            <NText>{{ testing.message }}</NText>
            <NCode v-if="testing.summary" :code="testing.summary" language="json" />
          </NSpace>
        </NAlert>
      </NForm>
    </div>
    <template #footer>
      <div class="modal-footer">
        <NText depth="3" class="test-note">测试配置当前仅发送文本测试消息。</NText>
        <NSpace justify="end">
          <NButton :loading="testing.loading" @click="testConfig">测试配置</NButton>
          <NButton @click="show = false">取消</NButton>
          <NButton type="primary" @click="submit">保存</NButton>
        </NSpace>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
.modal-body {
  max-height: min(68vh, 680px);
  overflow: auto;
  padding-right: 4px;
}

.form-section {
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  padding: 14px 14px 2px;
  background: var(--clay-surface-2);
}

.form-section + .form-section {
  margin-top: 12px;
}

.section-title {
  margin-bottom: 12px;
  color: var(--clay-text);
  font-size: 14px;
  font-weight: 800;
}

.sink-desc {
  margin-bottom: 14px;
}

.capability-panel {
  display: grid;
  gap: 10px;
  margin-bottom: 14px;
  padding: 12px;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface);
}

.cap-row {
  display: grid;
  grid-template-columns: 72px minmax(0, 1fr);
  gap: 10px;
  align-items: start;
}

.cap-label {
  color: var(--clay-text-muted);
  font-size: 13px;
  font-weight: 700;
}

.media-caps {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.media-cap-item {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  min-width: 0;
}

.cap-note {
  display: block;
  padding-left: 82px;
  font-size: 12px;
}

.test-result {
  margin-top: 12px;
}

.modal-footer {
  display: flex;
  flex-wrap: wrap;
  gap: 10px 16px;
  align-items: center;
  justify-content: space-between;
}

.test-note {
  font-size: 12px;
}
</style>
