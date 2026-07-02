<script setup lang="ts">
import { reactive, shallowRef, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { templatesApi } from '@/api/client'
import type { Template } from '@/types'
import { errText } from '@/utils/error'

const show = defineModel<boolean>('show', { required: true })

const props = defineProps<{
  template?: Template | null
}>()

const emit = defineEmits<{
  saved: []
}>()

const message = useMessage()

const form = reactive({ name: '', format: 'text', content: '{{.Text}}' })
const previewText = shallowRef('')
const previewing = shallowRef(false)

const formatOptions = [
  { label: 'text', value: 'text' },
  { label: 'markdown', value: 'markdown' },
  { label: 'html', value: 'html' },
]

const variables = [
  { label: '正文', value: '{{.Text}}' },
  { label: '发送者', value: '{{.SenderName}}' },
  { label: '消息类型', value: '{{.MessageType}}' },
  { label: '原文链接', value: '{{.OriginalURL}}' },
  { label: '来源 ID', value: '{{.SourceID}}' },
]

watch(
  () => [show.value, props.template] as const,
  () => {
    if (!show.value) return
    resetForm()
  },
  { immediate: true },
)

function resetForm() {
  if (props.template) {
    form.name = props.template.name
    form.format = props.template.format
    form.content = props.template.content
  } else {
    form.name = ''
    form.format = 'text'
    form.content = '{{.Text}}'
  }
  previewText.value = ''
}

function insertVariable(value: string) {
  form.content = form.content ? `${form.content}${value}` : value
}

function validate(): boolean {
  if (!form.name.trim()) {
    message.warning('请填写模板名称')
    return false
  }
  if (!form.content.trim()) {
    message.warning('请填写模板内容')
    return false
  }
  return true
}

async function preview() {
  previewing.value = true
  try {
    const result = await templatesApi.preview({ format: form.format, content: form.content })
    previewText.value = result.text
  } catch (e) {
    message.error('预览失败：' + errText(e))
  } finally {
    previewing.value = false
  }
}

async function submit() {
  if (!validate()) return
  try {
    if (props.template) {
      await templatesApi.update(props.template.id, { ...form })
    } else {
      await templatesApi.create({ ...form })
    }
    message.success('已保存模板')
    show.value = false
    emit('saved')
  } catch (e) {
    message.error('保存失败：' + errText(e))
  }
}
</script>

<template>
  <NModal v-model:show="show" preset="card" :title="template ? '编辑模板' : '新建模板'" class="template-modal">
    <NForm label-placement="left" label-width="72">
      <NFormItem label="名称" required>
        <NInput v-model:value="form.name" />
      </NFormItem>
      <NFormItem label="格式">
        <NSelect v-model:value="form.format" :options="formatOptions" />
      </NFormItem>
      <NFormItem label="变量">
        <NSpace>
          <NButton v-for="item in variables" :key="item.value" size="small" @click="insertVariable(item.value)">
            {{ item.label }}
          </NButton>
        </NSpace>
      </NFormItem>
      <NFormItem label="内容" required>
        <NInput v-model:value="form.content" type="textarea" :autosize="{ minRows: 7 }" />
      </NFormItem>
      <NDivider>预览</NDivider>
      <NSpace vertical>
        <NButton :loading="previewing" @click="preview">生成预览</NButton>
        <NInput :value="previewText" type="textarea" readonly :autosize="{ minRows: 5 }" />
      </NSpace>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="show = false">取消</NButton>
        <NButton type="primary" @click="submit">保存</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.template-modal {
  width: min(720px, calc(100vw - 32px));
}
</style>
