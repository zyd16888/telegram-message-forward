<script setup lang="ts">
import { computed, h, onMounted, reactive, ref } from 'vue'
import { NButton, NTag, useMessage, type DataTableColumns } from 'naive-ui'
import { aiApi, rulesApi, sinksApi, sourcesApi } from '@/api/client'
import type {
  AIDigestProfile,
  AIDigestProfileRequest,
  AIDigestRun,
  AIDigestRunDetail as AIDigestRunDetailType,
  AIProvider,
  RuleItemDescriptor,
  Sink,
  Source,
} from '@/types'
import { errText } from '@/utils/error'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'
import AIDigestProfileForm from '@/components/ai/AIDigestProfileForm.vue'
import AIDigestRunDetail from '@/components/ai/AIDigestRunDetail.vue'

const message = useMessage()

const loading = ref(false)
const providerLoading = ref(false)
const providerSaving = ref(false)
const providerTesting = ref(false)
const profiles = ref<AIDigestProfile[]>([])
const sources = ref<Source[]>([])
const sinks = ref<Sink[]>([])
const conditionDescriptors = ref<RuleItemDescriptor[]>([])
const selectedProfile = ref<AIDigestProfile | null>(null)
const showForm = ref(false)
const savingProfile = ref(false)
const previewing = ref(false)
const runs = ref<AIDigestRun[]>([])
const selectedDetail = ref<AIDigestRunDetailType | null>(null)
const showDetail = ref(false)

const provider = reactive<AIProvider & { api_key?: string }>({
  provider_type: 'openai_compatible',
  base_url: 'https://api.openai.com/v1',
  model: 'gpt-4o-mini',
  timeout_seconds: 60,
  max_retries: 1,
  default_temperature: 0.2,
  has_api_key: false,
  api_key: '',
})

const profileColumns: DataTableColumns<AIDigestProfile> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '名称', key: 'name' },
  {
    title: '状态',
    key: 'enabled',
    width: 110,
    render: (row) => h(NTag, { type: row.enabled ? 'success' : 'default', bordered: false }, { default: () => (row.enabled ? '已启用' : '手动') }),
  },
  {
    title: '输入/输出',
    key: 'io',
    render: (row) => `${row.source_ids.length} 个来源 / ${row.target_sink_ids.length} 个渠道`,
  },
  {
    title: '调度',
    key: 'schedule',
    render: (row) => scheduleLabel(row),
  },
  {
    title: '最近运行',
    key: 'recent_run',
    render: (row) => row.recent_run ? `${row.recent_run.status} · ${row.recent_run.created_at}` : '—',
  },
  {
    title: '操作',
    key: 'actions',
    width: 360,
    render: (row) =>
      h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', secondary: true, onClick: () => editProfile(row) }, { default: () => '编辑' }),
        h(NButton, { size: 'small', onClick: () => previewProfile(row) }, { default: () => '预览' }),
        h(NButton, { size: 'small', type: 'primary', onClick: () => runProfile(row) }, { default: () => '执行' }),
        h(NButton, { size: 'small', onClick: () => loadRuns(row) }, { default: () => '记录' }),
        h(NButton, { size: 'small', type: 'error', secondary: true, onClick: () => removeProfile(row) }, { default: () => '删除' }),
      ]),
  },
]

const runColumns: DataTableColumns<AIDigestRun> = [
  { title: 'ID', key: 'id', width: 70 },
  { title: '状态', key: 'status', width: 100 },
  { title: '触发', key: 'trigger_type', width: 100 },
  { title: '纳入/排除', key: 'counts', render: (row) => `${row.included_count}/${row.excluded_count}` },
  { title: 'Token', key: 'token', render: (row) => row.token_usage.total_tokens ?? 0 },
  { title: '创建时间', key: 'created_at' },
  {
    title: '操作',
    key: 'actions',
    width: 210,
    render: (row) =>
      h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', onClick: () => openRun(row.id) }, { default: () => '详情' }),
        h(NButton, { size: 'small', secondary: true, onClick: () => deliverRun(row.id) }, { default: () => '重新投递' }),
      ]),
  },
]

const activeProfileName = computed(() => selectedProfile.value?.name ?? '运行记录')

async function loadAll(): Promise<void> {
  loading.value = true
  try {
    const [ps, srcs, snks, meta, pvd] = await Promise.all([
      aiApi.profiles.list(),
      sourcesApi.list(),
      sinksApi.list(),
      rulesApi.meta(),
      aiApi.provider.get(),
    ])
    profiles.value = ps
    sources.value = srcs
    sinks.value = snks
    conditionDescriptors.value = meta.conditions.filter((item) => item.type !== 'source')
    Object.assign(provider, pvd, { api_key: '' })
    if (!selectedProfile.value && ps[0]) {
      selectedProfile.value = ps[0]
      await loadRuns(ps[0])
    }
  } catch (e) {
    message.error('加载 AI 整理失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

async function saveProvider(): Promise<void> {
  providerSaving.value = true
  try {
    const body = {
      provider_type: provider.provider_type,
      base_url: provider.base_url,
      model: provider.model,
      timeout_seconds: provider.timeout_seconds,
      max_retries: provider.max_retries,
      default_temperature: provider.default_temperature,
      api_key: provider.api_key?.trim() ? provider.api_key.trim() : undefined,
    }
    const res = await aiApi.provider.update(body)
    Object.assign(provider, res, { api_key: '' })
    message.success('Provider 设置已保存')
  } catch (e) {
    message.error('保存失败：' + errText(e))
  } finally {
    providerSaving.value = false
  }
}

async function testProvider(): Promise<void> {
  providerTesting.value = true
  try {
    const res = await aiApi.provider.test()
    if (res.success) message.success(res.text || 'Provider 测试成功')
    else message.error(res.error || 'Provider 测试失败')
  } catch (e) {
    message.error('测试失败：' + errText(e))
  } finally {
    providerTesting.value = false
  }
}

function newProfile(): void {
  selectedProfile.value = null
  showForm.value = true
}

function editProfile(profile: AIDigestProfile): void {
  selectedProfile.value = profile
  showForm.value = true
}

async function saveProfile(payload: AIDigestProfileRequest): Promise<void> {
  savingProfile.value = true
  try {
    if (selectedProfile.value?.id) {
      await aiApi.profiles.update(selectedProfile.value.id, payload)
    } else {
      await aiApi.profiles.create(payload)
    }
    message.success('AI Profile 已保存')
    showForm.value = false
    await loadAll()
  } catch (e) {
    message.error('保存失败：' + errText(e))
  } finally {
    savingProfile.value = false
  }
}

async function previewDraft(payload: AIDigestProfileRequest): Promise<void> {
  previewing.value = true
  try {
    selectedDetail.value = await aiApi.profiles.previewDraft(payload)
    showDetail.value = true
  } catch (e) {
    message.error('预览失败：' + errText(e))
  } finally {
    previewing.value = false
  }
}

async function previewProfile(profile: AIDigestProfile): Promise<void> {
  try {
    selectedDetail.value = await aiApi.profiles.preview(profile.id)
    showDetail.value = true
    await loadRuns(profile)
  } catch (e) {
    message.error('预览失败：' + errText(e))
  }
}

async function runProfile(profile: AIDigestProfile): Promise<void> {
  try {
    selectedDetail.value = await aiApi.profiles.run(profile.id)
    showDetail.value = true
    await loadAll()
    await loadRuns(profile)
  } catch (e) {
    message.error('执行失败：' + errText(e))
  }
}

async function loadRuns(profile: AIDigestProfile): Promise<void> {
  selectedProfile.value = profile
  runs.value = await aiApi.profiles.runs(profile.id)
}

async function openRun(id: number): Promise<void> {
  selectedDetail.value = await aiApi.runs.get(id)
  showDetail.value = true
}

async function deliverRun(id: number): Promise<void> {
  try {
    const res = await aiApi.runs.deliver(id)
    message.success(`已创建投递任务：${res.delivery_task_ids.join(', ') || '无可用渠道'}`)
    await openRun(id)
  } catch (e) {
    message.error('重新投递失败：' + errText(e))
  }
}

async function removeProfile(profile: AIDigestProfile): Promise<void> {
  try {
    await aiApi.profiles.remove(profile.id)
    message.success('已删除')
    if (selectedProfile.value?.id === profile.id) {
      selectedProfile.value = null
      runs.value = []
    }
    await loadAll()
  } catch (e) {
    message.error('删除失败：' + errText(e))
  }
}

async function cleanupRuns(): Promise<void> {
  try {
    const res = await aiApi.runs.cleanup(30)
    message.success(`已清理 ${res.deleted} 条 30 天前运行记录`)
    if (selectedProfile.value) await loadRuns(selectedProfile.value)
  } catch (e) {
    message.error('清理失败：' + errText(e))
  }
}

function scheduleLabel(profile: AIDigestProfile): string {
  const schedule = profile.schedule
  if (schedule.type === 'interval') return `每 ${schedule.interval_minutes || 60} 分钟`
  if (schedule.type === 'daily') return `每日 ${schedule.time || '09:00'} ${schedule.timezone || ''}`
  return '仅手动'
}

onMounted(loadAll)
</script>

<template>
  <NSpace vertical size="large">
    <PageHeader title="AI 整理" desc="按窗口收集消息、生成 briefing，并投递到指定渠道" icon="ai" />

    <NCard title="AI Provider">
      <template #header-extra>
        <NTag :type="provider.has_api_key ? 'success' : 'warning'" :bordered="false">
          {{ provider.has_api_key ? '已配置 Key' : '未配置 Key' }}
        </NTag>
      </template>
      <NSpin :show="providerLoading">
        <NForm label-placement="left" label-width="130" :show-feedback="false">
          <div class="provider-grid">
            <NFormItem label="类型">
              <NInput v-model:value="provider.provider_type" disabled />
            </NFormItem>
            <NFormItem label="Base URL">
              <NInput v-model:value="provider.base_url" placeholder="https://api.openai.com/v1" />
            </NFormItem>
            <NFormItem label="默认模型">
              <NInput v-model:value="provider.model" placeholder="gpt-4o-mini" />
            </NFormItem>
            <NFormItem label="API Key">
              <NInput
                v-model:value="provider.api_key"
                type="password"
                show-password-on="click"
                :placeholder="provider.has_api_key ? '已配置，留空表示不修改' : 'sk-...'"
              />
            </NFormItem>
            <NFormItem label="超时秒数">
              <NInputNumber v-model:value="provider.timeout_seconds" :min="5" class="full-input" />
            </NFormItem>
            <NFormItem label="重试次数">
              <NInputNumber v-model:value="provider.max_retries" :min="0" class="full-input" />
            </NFormItem>
            <NFormItem label="Temperature">
              <NInputNumber v-model:value="provider.default_temperature" :min="0" :max="2" :step="0.1" class="full-input" />
            </NFormItem>
          </div>
        </NForm>
        <NSpace>
          <NButton type="primary" :loading="providerSaving" @click="saveProvider">保存 Provider</NButton>
          <NButton secondary :loading="providerTesting" @click="testProvider">测试 Provider</NButton>
        </NSpace>
      </NSpin>
    </NCard>

    <NCard title="Profile">
      <template #header-extra>
        <NSpace>
          <NButton secondary @click="cleanupRuns">清理运行记录</NButton>
          <NButton type="primary" @click="newProfile">
            <template #icon><ClayIcon name="plus" :size="15" /></template>
            新建 Profile
          </NButton>
        </NSpace>
      </template>
      <NDataTable :loading="loading" :columns="profileColumns" :data="profiles" :bordered="false" :scroll-x="980" />
    </NCard>

    <NCard :title="activeProfileName">
      <NDataTable :columns="runColumns" :data="runs" :bordered="false" :scroll-x="760" />
    </NCard>

    <AIDigestProfileForm
      v-model:show="showForm"
      :profile="selectedProfile"
      :sources="sources"
      :sinks="sinks"
      :condition-descriptors="conditionDescriptors"
      :saving="savingProfile"
      :previewing="previewing"
      @save="saveProfile"
      @preview="previewDraft"
    />

    <NModal
      v-model:show="showDetail"
      preset="card"
      title="AI 运行详情"
      :style="{ width: 'min(1000px, calc(100vw - 32px))' }"
    >
      <AIDigestRunDetail :detail="selectedDetail" />
    </NModal>
  </NSpace>
</template>

<style scoped>
.provider-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(180px, 1fr));
  gap: 12px;
  align-items: start;
}

.full-input {
  width: 100%;
}

:deep(.row-actions) {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

@media (max-width: 900px) {
  .provider-grid {
    grid-template-columns: 1fr;
  }
}
</style>
