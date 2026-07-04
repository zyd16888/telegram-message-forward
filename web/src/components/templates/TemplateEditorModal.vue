<script setup lang="ts">
import { computed, reactive, shallowRef, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
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
// 记录生成预览时所用的格式，避免预览后切换格式导致渲染方式与内容不一致。
const previewFormat = shallowRef('text')
const previewing = shallowRef(false)
const previewTab = shallowRef<'rendered' | 'raw'>('rendered')

const previewHtml = computed(() => {
  if (!previewText.value) return ''
  if (previewFormat.value === 'markdown') {
    return DOMPurify.sanitize(marked.parse(previewText.value, { async: false }))
  }
  if (previewFormat.value === 'html') {
    return DOMPurify.sanitize(previewText.value)
  }
  return ''
})

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
  previewFormat.value = form.format
  previewTab.value = 'rendered'
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
  await previewContent(true)
}

async function previewContent(showSuccess: boolean): Promise<boolean> {
  previewing.value = true
  try {
    const result = await templatesApi.preview({ format: form.format, content: form.content })
    previewText.value = result.text
    previewFormat.value = form.format
    if (showSuccess) message.success('预览已生成')
    return true
  } catch (e) {
    message.error('预览失败：' + errText(e))
    return false
  } finally {
    previewing.value = false
  }
}

async function submit() {
  if (!validate()) return
  if (!(await previewContent(false))) return
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
  <NModal
    v-model:show="show"
    preset="card"
    :title="template ? '编辑模板' : '新建模板'"
    class="template-modal"
    :style="{ width: 'min(680px, calc(100vw - 32px))' }"
  >
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
        <NSpace align="center" justify="space-between">
          <NButton :loading="previewing" @click="preview">生成预览</NButton>
          <NRadioGroup v-if="previewText && previewFormat !== 'text'" v-model:value="previewTab" size="small">
            <NRadioButton value="rendered">渲染效果</NRadioButton>
            <NRadioButton value="raw">原始输出</NRadioButton>
          </NRadioGroup>
        </NSpace>
        <NEmpty v-if="!previewText" size="small" description="点击「生成预览」查看模板渲染结果" />
        <template v-else>
          <div
            v-if="previewFormat !== 'text' && previewTab === 'rendered'"
            class="preview-rendered"
            v-html="previewHtml"
          />
          <div v-else class="preview-plain">{{ previewText }}</div>
        </template>
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
.template-modal :deep(.n-card__content) {
  max-height: min(68vh, 680px);
  overflow: auto;
}

.preview-rendered,
.preview-plain {
  max-height: 280px;
  overflow: auto;
  padding: 12px;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  background: var(--clay-surface-2);
  font-size: 13px;
  line-height: 1.6;
  word-break: break-word;
}

.preview-plain {
  white-space: pre-wrap;
}

.preview-rendered :deep(p) {
  margin: 0 0 8px;
}

.preview-rendered :deep(p:last-child) {
  margin-bottom: 0;
}

.preview-rendered :deep(pre) {
  overflow: auto;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--clay-surface);
}

.preview-rendered :deep(code) {
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 12px;
}

.preview-rendered :deep(blockquote) {
  margin: 0 0 8px;
  padding: 4px 12px;
  border-left: 3px solid var(--clay-border-strong);
  color: var(--clay-text-2);
}

.preview-rendered :deep(img) {
  max-width: 100%;
}

.preview-rendered :deep(h1),
.preview-rendered :deep(h2),
.preview-rendered :deep(h3),
.preview-rendered :deep(h4) {
  margin: 0 0 8px;
  line-height: 1.4;
}

.preview-rendered :deep(ul),
.preview-rendered :deep(ol) {
  margin: 0 0 8px;
  padding-left: 20px;
}

.preview-rendered :deep(table) {
  margin: 0 0 8px;
  border-collapse: collapse;
}

.preview-rendered :deep(th),
.preview-rendered :deep(td) {
  padding: 4px 8px;
  border: 1px solid var(--clay-border);
}
</style>
