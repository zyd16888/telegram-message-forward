<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useMessage } from 'naive-ui'
import ConfigFormRenderer from '@/components/ConfigFormRenderer.vue'
import { sinksApi } from '@/api/client'
import type { Sink, SinkDescriptor } from '@/types'
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
  } catch (e) {
    testing.success = false
    testing.message = errText(e)
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
  <NModal v-model:show="show" preset="card" :title="editing ? '编辑渠道' : '新建渠道'" class="sink-modal">
    <NForm label-placement="left" label-width="96">
      <NFormItem label="类型" required>
        <NSelect v-model:value="form.type" :disabled="editing" :options="typeOptions" />
      </NFormItem>
      <NFormItem label="名称" required>
        <NInput v-model:value="form.name" placeholder="例如：技术群通知" />
      </NFormItem>
      <NFormItem label="启用">
        <NSwitch v-model:value="form.enabled" />
      </NFormItem>
      <NDivider>渠道配置</NDivider>
      <NAlert v-if="descriptor?.description" type="default" :show-icon="false" class="sink-desc">
        {{ descriptor.description }}
      </NAlert>
      <ConfigFormRenderer v-if="descriptor" v-model="form.config" :fields="descriptor.config_fields" />
      <NFormItem v-if="descriptor?.secret_field" :label="descriptor.secret_field.label" :required="!editing && descriptor.secret_field.required">
        <NInput
          v-model:value="form.secret"
          type="password"
          show-password-on="click"
          :placeholder="editing ? '留空则保留原密钥' : descriptor.secret_field.placeholder"
        />
      </NFormItem>
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
    <template #footer>
      <NSpace justify="end">
        <NButton :loading="testing.loading" @click="testConfig">测试配置</NButton>
        <NButton @click="show = false">取消</NButton>
        <NButton type="primary" @click="submit">保存</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.sink-modal {
  width: min(680px, calc(100vw - 32px));
}

.sink-desc {
  margin-bottom: 14px;
}

.test-result {
  margin-top: 12px;
}
</style>
