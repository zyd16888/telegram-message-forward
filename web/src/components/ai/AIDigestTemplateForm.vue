<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { useMessage } from 'naive-ui'
import { aiApi } from '@/api/client'
import type { AIDigestOutputTemplate, AIDigestOutputTemplateRequest } from '@/types'
import { errText } from '@/utils/error'

const show = defineModel<boolean>('show', { required: true })

const props = defineProps<{ template?: AIDigestOutputTemplate | null }>()
const emit = defineEmits<{ saved: [] }>()

const message = useMessage()
const saving = ref(false)

const formatOptions = [
  { label: 'Markdown', value: 'markdown' },
  { label: 'Text', value: 'text' },
  { label: 'HTML', value: 'html' },
]

const form = reactive<AIDigestOutputTemplateRequest>({
  name: '',
  description: '',
  format: 'markdown',
  content: '',
})

watch(
  () => [show.value, props.template] as const,
  () => {
    if (!show.value) return
    if (props.template) {
      form.name = props.template.name
      form.description = props.template.description
      form.format = props.template.format || 'markdown'
      form.content = props.template.content
    } else {
      form.name = ''
      form.description = ''
      form.format = 'markdown'
      form.content = defaultContent()
    }
  },
  { immediate: true },
)

function defaultContent(): string {
  return '# {{profile_name}}\n\n## 一句话总结\n用 1 段话概括本窗口最重要的信息。\n\n## 重点摘要\n- 列出 3-7 条重点，每条都标注来源编号，例如 [#1]。\n\n## 分类整理\n按主题分组整理，每组包含关键事实、背景线索和来源编号。\n\n## 待关注事项\n列出需要继续关注的问题、风险、待办或后续进展。'
}

async function save(): Promise<void> {
  if (!form.name.trim()) {
    message.warning('请填写模板名称')
    return
  }
  if (!form.content.trim()) {
    message.warning('请填写模板内容')
    return
  }
  saving.value = true
  try {
    const body: AIDigestOutputTemplateRequest = {
      name: form.name.trim(),
      description: form.description.trim(),
      format: form.format,
      content: form.content,
    }
    if (props.template?.id) {
      await aiApi.outputTemplates.update(props.template.id, body)
    } else {
      await aiApi.outputTemplates.create(body)
    }
    message.success('输出模板已保存')
    show.value = false
    emit('saved')
  } catch (e) {
    message.error('保存失败：' + errText(e))
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <NModal
    v-model:show="show"
    preset="card"
    :title="template ? '编辑输出模板' : '新建输出模板'"
    :style="{ width: 'min(760px, calc(100vw - 32px))' }"
  >
    <NForm label-placement="top" :show-feedback="false">
      <div class="meta-grid">
        <NFormItem label="模板名称" required>
          <NInput v-model:value="form.name" placeholder="例如：群聊摘要" />
        </NFormItem>
        <NFormItem label="输出格式">
          <NSelect v-model:value="form.format" :options="formatOptions" />
        </NFormItem>
      </div>
      <NFormItem label="用途说明">
        <NInput v-model:value="form.description" placeholder="一句话说明适用场景，便于在 Profile 中挑选" />
      </NFormItem>
      <NFormItem label="结构内容" required>
        <NInput
          v-model:value="form.content"
          type="textarea"
          placeholder="定义 AI 输出的标题、章节、列表、来源标注等结构规范；可用 {{profile_name}} 等变量"
          :autosize="{ minRows: 12, maxRows: 22 }"
        />
      </NFormItem>
      <NAlert type="default" :show-icon="false" class="hint">
        这里定义「内容长什么样」（标题、摘要、分类、待办、来源标注等）；上面的输出格式只决定用 Markdown / Text / HTML 作为载体。可在多个 Profile 中复用同一模板。
      </NAlert>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="show = false">取消</NButton>
        <NButton type="primary" :loading="saving" @click="save">保存</NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.meta-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 200px;
  gap: 12px;
  align-items: start;
}

.hint {
  margin-top: 4px;
}

@media (max-width: 620px) {
  .meta-grid {
    grid-template-columns: 1fr;
  }
}
</style>
