<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, reactive, ref, shallowRef } from 'vue'
import { NButton, NTag, NText, useDialog, useMessage, type DataTableColumns } from 'naive-ui'
import { useRouter } from 'vue-router'
import { aiApi, filtersApi, flowsApi, sinksApi, sourcesApi } from '@/api/client'
import type {
  AIDigestProfile,
  AIDigestProfileRequest,
  AIDigestPreset,
  AIDigestOutputTemplate,
  AIDigestRun,
  AIDigestRunDetail as AIDigestRunDetailType,
  AIProvider,
  AIProviderRequest,
  Filter,
  RuleItemDescriptor,
  Sink,
  Source,
} from '@/types'
import { errText } from '@/utils/error'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'
import AIDigestProfileForm from '@/components/ai/AIDigestProfileForm.vue'
import AIDigestTemplateForm from '@/components/ai/AIDigestTemplateForm.vue'
import AIDigestRunDetail from '@/components/ai/AIDigestRunDetail.vue'
import AIDigestScheduleStatus from '@/components/ai/AIDigestScheduleStatus.vue'

const message = useMessage()
const dialog = useDialog()
const router = useRouter()

const activeTab = ref<'profiles' | 'templates' | 'providers'>('profiles')

const loading = ref(false)
const providerSaving = ref(false)
const providerTesting = ref(false)
const editingProviderId = ref<string | null>(null)
const showProviderForm = ref(false)
const providers = ref<AIProvider[]>([])
const presets = ref<AIDigestPreset[]>([])
const templates = ref<AIDigestOutputTemplate[]>([])
const filters = ref<Filter[]>([])
const profiles = ref<AIDigestProfile[]>([])
const sources = ref<Source[]>([])
const sinks = ref<Sink[]>([])
const conditionDescriptors = ref<RuleItemDescriptor[]>([])
const scheduleNow = shallowRef(Date.now())
const selectedProfile = ref<AIDigestProfile | null>(null)
const showForm = ref(false)
const savingProfile = ref(false)
const previewing = ref(false)

const showTemplateForm = ref(false)
const editingTemplate = ref<AIDigestOutputTemplate | null>(null)

const runs = ref<AIDigestRun[]>([])
const runsProfile = ref<AIDigestProfile | null>(null)
const showRunsDrawer = ref(false)
const runsLoading = ref(false)

const selectedDetail = ref<AIDigestRunDetailType | null>(null)
const showDetail = ref(false)
const detailLoading = ref(false)

const providerForm = reactive<AIProviderRequest>(defaultProviderForm())

const apiTypeOptions = [
  { label: 'Chat Completions (/v1/chat/completions)', value: 'chat_completions' },
  { label: 'Responses API (/v1/responses)', value: 'responses' },
]

const templateNameById = computed(() => {
  const map = new Map<number, string>()
  for (const t of templates.value) map.set(t.id, t.name)
  return map
})

const providerFormTitle = computed(() => (editingProviderId.value ? '编辑 AI Provider' : '新增 AI Provider'))
const editingProvider = computed(() => providers.value.find((item) => item.id === editingProviderId.value) ?? null)

// --- 列定义 ---
const profileColumns: DataTableColumns<AIDigestProfile> = [
  {
    title: '名称',
    key: 'name',
    render: (row) =>
      h('div', { class: 'cell-stack' }, [
        h('span', { class: 'cell-title' }, row.name),
        h(NTag, { size: 'small', type: row.enabled ? 'success' : 'default', bordered: false }, { default: () => (row.enabled ? '定时启用' : '仅手动') }),
      ]),
  },
  {
    title: '输入 / 输出',
    key: 'io',
    render: (row) => `${row.source_ids.length} 来源 · ${row.target_sink_ids.length} 渠道`,
  },
  {
    title: '输出模板',
    key: 'template',
    render: (row) =>
      row.output_template_id
        ? h(NTag, { size: 'small', type: 'info', bordered: false }, { default: () => templateNameById.value.get(row.output_template_id) ?? `#${row.output_template_id}` })
        : h(NText, { depth: 3 }, { default: () => '自定义' }),
  },
  {
    title: '调度',
    key: 'schedule',
    width: 240,
    render: (row) => h(AIDigestScheduleStatus, { schedule: row.schedule, enabled: row.enabled, now: scheduleNow.value }),
  },
  {
    title: '最近运行',
    key: 'recent_run',
    render: (row) => {
      const run = row.recent_run
      if (!run) return h(NText, { depth: 3 }, { default: () => '—' })
      return h(NTag, { size: 'small', type: runStatusType(run.status), bordered: false }, { default: () => `${runStatusLabel(run.status)} · ${run.created_at}` })
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 300,
    render: (row) =>
      h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', type: 'primary', secondary: true, onClick: () => runProfile(row) }, { default: () => '执行' }),
        h(NButton, { size: 'small', onClick: () => previewProfile(row) }, { default: () => '预览' }),
        h(NButton, { size: 'small', onClick: () => openRunsDrawer(row) }, { default: () => '记录' }),
        h(NButton, { size: 'small', secondary: true, onClick: () => editProfile(row) }, { default: () => '编辑' }),
        h(NButton, { size: 'small', type: 'error', quaternary: true, onClick: () => confirmRemoveProfile(row) }, { default: () => '删除' }),
      ]),
  },
]

const templateColumns: DataTableColumns<AIDigestOutputTemplate> = [
  {
    title: '名称',
    key: 'name',
    render: (row) =>
      h('div', { class: 'cell-stack' }, [
        h('span', { class: 'cell-title' }, row.name),
        row.built_in ? h(NTag, { size: 'small', type: 'warning', bordered: false }, { default: () => '内置' }) : null,
      ]),
  },
  { title: '用途', key: 'description', ellipsis: { tooltip: true }, render: (row) => row.description || '—' },
  { title: '格式', key: 'format', width: 110, render: (row) => row.format.toUpperCase() },
  {
    title: '引用',
    key: 'used',
    width: 90,
    render: (row) => {
      const count = profiles.value.filter((p) => p.output_template_id === row.id).length
      return count ? h(NText, {}, { default: () => `${count} 个` }) : h(NText, { depth: 3 }, { default: () => '未使用' })
    },
  },
  {
    title: '操作',
    key: 'actions',
    width: 170,
    render: (row) =>
      h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', secondary: true, onClick: () => editTemplate(row) }, { default: () => '编辑' }),
        h(NButton, { size: 'small', type: 'error', quaternary: true, disabled: row.built_in, onClick: () => confirmRemoveTemplate(row) }, { default: () => '删除' }),
      ]),
  },
]

const providerColumns: DataTableColumns<AIProvider> = [
  {
    title: '名称',
    key: 'name',
    render: (row) =>
      h('div', { class: 'cell-stack' }, [
        h('span', { class: 'cell-title' }, row.name || row.id),
        row.is_default ? h(NTag, { size: 'small', type: 'success', bordered: false }, { default: () => '默认' }) : null,
      ]),
  },
  { title: '接口', key: 'api_type', width: 130, render: (row) => (row.api_type === 'responses' ? 'Responses' : 'Chat') },
  { title: 'Base URL', key: 'base_url', ellipsis: { tooltip: true } },
  { title: '默认模型', key: 'model', width: 150 },
  {
    title: 'Key',
    key: 'has_api_key',
    width: 90,
    render: (row) => h(NTag, { size: 'small', type: row.has_api_key ? 'success' : 'warning', bordered: false }, { default: () => (row.has_api_key ? '已配置' : '未配置') }),
  },
  {
    title: '操作',
    key: 'actions',
    width: 210,
    render: (row) =>
      h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', secondary: true, onClick: () => editProvider(row) }, { default: () => '编辑' }),
        h(NButton, { size: 'small', loading: providerTesting.value, onClick: () => testProvider(row) }, { default: () => '测试' }),
        h(NButton, { size: 'small', type: 'error', quaternary: true, disabled: providers.value.length <= 1, onClick: () => confirmRemoveProvider(row) }, { default: () => '删除' }),
      ]),
  },
]

const runColumns: DataTableColumns<AIDigestRun> = [
  { title: 'ID', key: 'id', width: 60 },
  {
    title: '状态',
    key: 'status',
    width: 90,
    render: (row) => h(NTag, { size: 'small', type: runStatusType(row.status), bordered: false }, { default: () => runStatusLabel(row.status) }),
  },
  { title: '触发', key: 'trigger_type', width: 80, render: (row) => triggerLabel(row.trigger_type) },
  { title: '纳入/排除', key: 'counts', width: 90, render: (row) => `${row.included_count}/${row.excluded_count}` },
  { title: 'Provider', key: 'provider_name', render: (row) => row.provider_name || '—' },
  { title: 'Token', key: 'token', width: 80, render: (row) => row.token_usage.total_tokens ?? 0 },
  { title: '时间', key: 'created_at', width: 160 },
  {
    title: '操作',
    key: 'actions',
    width: 170,
    render: (row) =>
      h('div', { class: 'row-actions' }, [
        h(NButton, { size: 'small', secondary: true, onClick: () => openRun(row.id) }, { default: () => '详情' }),
        h(NButton, { size: 'small', onClick: () => deliverRun(row.id) }, { default: () => '重新投递' }),
      ]),
  },
]

// --- 加载 ---
async function loadAll(): Promise<void> {
  loading.value = true
  try {
    const [ps, srcs, snks, meta, pvds, presetItems, tmpls, flts] = await Promise.all([
      aiApi.profiles.list(),
      sourcesApi.list(),
      sinksApi.list(),
      flowsApi.meta(),
      aiApi.providers.list(),
      aiApi.presets.list(),
      aiApi.outputTemplates.list(),
      filtersApi.list(),
    ])
    profiles.value = ps
    sources.value = srcs
    sinks.value = snks
    conditionDescriptors.value = meta.conditions.filter((item) => item.type !== 'source')
    providers.value = pvds
    presets.value = presetItems
    templates.value = tmpls
    filters.value = flts
  } catch (e) {
    message.error('加载 AI 整理失败：' + errText(e))
  } finally {
    loading.value = false
  }
}

let scheduleTimer: number | null = null
let scheduleRefreshing = false

async function refreshScheduleStatus(): Promise<void> {
  if (scheduleRefreshing) return
  scheduleRefreshing = true
  scheduleNow.value = Date.now()
  try {
    profiles.value = await aiApi.profiles.list()
  } catch {
    // 保留当前数据，主刷新按钮仍会展示请求错误。
  } finally {
    scheduleRefreshing = false
  }
}

async function loadProviders(): Promise<void> {
  providers.value = await aiApi.providers.list()
}

async function loadTemplates(): Promise<void> {
  templates.value = await aiApi.outputTemplates.list()
}

// --- Provider ---
function newProvider(): void {
  editingProviderId.value = null
  Object.assign(providerForm, defaultProviderForm())
  showProviderForm.value = true
}

function editProvider(provider: AIProvider): void {
  editingProviderId.value = provider.id
  Object.assign(providerForm, {
    name: provider.name,
    provider_type: provider.provider_type,
    api_type: provider.api_type || 'chat_completions',
    base_url: provider.base_url,
    model: provider.model,
    timeout_seconds: provider.timeout_seconds,
    max_retries: provider.max_retries,
    default_temperature: provider.default_temperature,
    enabled: true,
    is_default: provider.is_default,
    api_key: '',
  })
  showProviderForm.value = true
}

function defaultProviderForm(): AIProviderRequest {
  return {
    name: '',
    provider_type: 'openai_compatible',
    api_type: 'chat_completions',
    base_url: 'https://api.openai.com/v1',
    model: 'gpt-4o-mini',
    timeout_seconds: 60,
    max_retries: 1,
    default_temperature: 0.2,
    enabled: true,
    is_default: providers.value.length === 0,
    api_key: '',
  }
}

function providerPayload(): AIProviderRequest {
  return {
    name: providerForm.name,
    provider_type: providerForm.provider_type,
    api_type: providerForm.api_type,
    base_url: providerForm.base_url,
    model: providerForm.model,
    timeout_seconds: providerForm.timeout_seconds,
    max_retries: providerForm.max_retries,
    default_temperature: providerForm.default_temperature,
    enabled: true,
    is_default: providerForm.is_default,
    api_key: providerForm.api_key?.trim() ? providerForm.api_key.trim() : undefined,
  }
}

async function saveProvider(): Promise<void> {
  providerSaving.value = true
  try {
    const body = providerPayload()
    const res = editingProviderId.value
      ? await aiApi.providers.update(editingProviderId.value, body)
      : await aiApi.providers.create(body)
    editingProviderId.value = res.id
    await loadProviders()
    showProviderForm.value = false
    message.success('Provider 设置已保存')
  } catch (e) {
    message.error('保存失败：' + errText(e))
  } finally {
    providerSaving.value = false
  }
}

async function testProvider(provider?: AIProvider): Promise<void> {
  providerTesting.value = true
  try {
    const res = provider
      ? await aiApi.providers.test(provider.id)
      : editingProviderId.value
        ? await aiApi.providers.test(editingProviderId.value, providerPayload())
        : await aiApi.providers.testDraft(providerPayload())
    if (res.success) message.success(res.text || 'Provider 测试成功')
    else message.error(res.error || 'Provider 测试失败')
  } catch (e) {
    message.error('测试失败：' + errText(e))
  } finally {
    providerTesting.value = false
  }
}

function confirmRemoveProvider(provider: AIProvider): void {
  dialog.warning({
    title: '删除 Provider',
    content: `确定删除「${provider.name || provider.id}」？`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await aiApi.providers.remove(provider.id)
        message.success('已删除 Provider')
        editingProviderId.value = null
        await loadProviders()
      } catch (e) {
        message.error('删除失败：' + errText(e))
      }
    },
  })
}

// --- 输出模板 ---
function newTemplate(): void {
  editingTemplate.value = null
  showTemplateForm.value = true
}

function editTemplate(t: AIDigestOutputTemplate): void {
  editingTemplate.value = t
  showTemplateForm.value = true
}

function confirmRemoveTemplate(t: AIDigestOutputTemplate): void {
  dialog.warning({
    title: '删除输出模板',
    content: `确定删除模板「${t.name}」？被 Profile 引用时无法删除。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await aiApi.outputTemplates.remove(t.id)
        message.success('已删除模板')
        await loadTemplates()
      } catch (e) {
        message.error('删除失败：' + errText(e))
      }
    },
  })
}

// --- Profile ---
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
  detailLoading.value = true
  selectedDetail.value = null
  showDetail.value = true
  try {
    selectedDetail.value = await aiApi.profiles.previewDraft(payload)
  } catch (e) {
    message.error('预览失败：' + errText(e))
  } finally {
    detailLoading.value = false
    previewing.value = false
  }
}

async function previewProfile(profile: AIDigestProfile): Promise<void> {
  detailLoading.value = true
  selectedDetail.value = null
  showDetail.value = true
  try {
    selectedDetail.value = await aiApi.profiles.preview(profile.id)
  } catch (e) {
    message.error('预览失败：' + errText(e))
  } finally {
    detailLoading.value = false
  }
}

async function runProfile(profile: AIDigestProfile): Promise<void> {
  detailLoading.value = true
  selectedDetail.value = null
  showDetail.value = true
  try {
    selectedDetail.value = await aiApi.profiles.run(profile.id)
    await loadAll()
    if (showRunsDrawer.value && runsProfile.value?.id === profile.id) await refreshRuns()
  } catch (e) {
    message.error('执行失败：' + errText(e))
  } finally {
    detailLoading.value = false
  }
}

function confirmRemoveProfile(profile: AIDigestProfile): void {
  dialog.warning({
    title: '删除 Profile',
    content: `确定删除「${profile.name}」？相关运行记录会一并删除。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await aiApi.profiles.remove(profile.id)
        message.success('已删除')
        await loadAll()
      } catch (e) {
        message.error('删除失败：' + errText(e))
      }
    },
  })
}

// --- 运行记录抽屉 ---
async function openRunsDrawer(profile: AIDigestProfile): Promise<void> {
  runsProfile.value = profile
  showRunsDrawer.value = true
  await refreshRuns()
}

async function refreshRuns(): Promise<void> {
  if (!runsProfile.value) return
  runsLoading.value = true
  try {
    runs.value = await aiApi.profiles.runs(runsProfile.value.id)
  } catch (e) {
    message.error('加载运行记录失败：' + errText(e))
  } finally {
    runsLoading.value = false
  }
}

async function openRun(id: number): Promise<void> {
  detailLoading.value = true
  selectedDetail.value = null
  showDetail.value = true
  try {
    selectedDetail.value = await aiApi.runs.get(id)
  } catch (e) {
    message.error('加载运行详情失败：' + errText(e))
  } finally {
    detailLoading.value = false
  }
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

function cleanupRuns(): void {
  dialog.warning({
    title: '清理运行记录',
    content: '将删除所有 30 天前的运行记录，确定继续？',
    positiveText: '清理',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        const res = await aiApi.runs.cleanup(30)
        message.success(`已清理 ${res.deleted} 条 30 天前运行记录`)
        if (showRunsDrawer.value) await refreshRuns()
      } catch (e) {
        message.error('清理失败：' + errText(e))
      }
    },
  })
}

// --- 展示辅助 ---
function runStatusLabel(status: string): string {
  return { success: '成功', failed: '失败', running: '运行中', pending: '排队', cancelled: '已取消' }[status] ?? status
}

function runStatusType(status: string): 'success' | 'error' | 'info' | 'warning' | 'default' {
  return ({ success: 'success', failed: 'error', running: 'info', pending: 'warning', cancelled: 'default' } as const)[status] ?? 'default'
}

function triggerLabel(trigger: string): string {
  return { manual: '手动', schedule: '定时', preview: '预览' }[trigger] ?? trigger
}

onMounted(() => {
  void loadAll()
  scheduleTimer = window.setInterval(() => void refreshScheduleStatus(), 60_000)
})

onBeforeUnmount(() => {
  if (scheduleTimer !== null) window.clearInterval(scheduleTimer)
})
</script>

<template>
  <NSpace vertical size="large">
    <PageHeader title="AI 整理" desc="按窗口收集消息、生成 briefing，并投递到指定渠道" icon="ai">
      <template #actions>
        <NButton v-if="activeTab === 'profiles'" type="primary" @click="newProfile">
          <template #icon><ClayIcon name="plus" :size="16" /></template>
          新建 Profile
        </NButton>
        <NButton v-else-if="activeTab === 'templates'" type="primary" @click="newTemplate">
          <template #icon><ClayIcon name="plus" :size="16" /></template>
          新建模板
        </NButton>
        <NButton v-else type="primary" @click="newProvider">
          <template #icon><ClayIcon name="plus" :size="16" /></template>
          新增 Provider
        </NButton>
        <NButton secondary @click="loadAll">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </NButton>
      </template>
    </PageHeader>

    <NTabs v-model:value="activeTab" type="segment" size="large" animated>
      <NTabPane name="profiles" tab="整理任务">
        <NDataTable
          :loading="loading"
          :columns="profileColumns"
          :data="profiles"
          :bordered="false"
          :scroll-x="1000"
          :row-key="(row: AIDigestProfile) => row.id"
        >
          <template #empty>
            <div class="tab-empty">
              <p>还没有 AI 整理任务。</p>
              <NButton type="primary" size="small" @click="newProfile">新建第一个 Profile</NButton>
            </div>
          </template>
        </NDataTable>
      </NTabPane>

      <NTabPane name="templates" tab="输出模板">
        <div class="tab-lead">
          输出模板定义 AI 结果的结构（标题、章节、来源标注等），可被多个 Profile 复用。
        </div>
        <NDataTable
          :loading="loading"
          :columns="templateColumns"
          :data="templates"
          :bordered="false"
          :scroll-x="720"
          :row-key="(row: AIDigestOutputTemplate) => row.id"
        >
          <template #empty>
            <div class="tab-empty">
              <p>还没有输出模板。</p>
              <NButton type="primary" size="small" @click="newTemplate">新建模板</NButton>
            </div>
          </template>
        </NDataTable>
      </NTabPane>

      <NTabPane name="providers" tab="AI 服务">
        <div class="tab-lead">配置 OpenAI 兼容的 AI Provider，Profile 执行时会调用选定的服务。</div>
        <NDataTable
          :loading="loading"
          :columns="providerColumns"
          :data="providers"
          :bordered="false"
          :scroll-x="900"
          :row-key="(row: AIProvider) => row.id"
        />
      </NTabPane>
    </NTabs>

    <!-- Profile 表单 -->
    <AIDigestProfileForm
      v-model:show="showForm"
      :profile="selectedProfile"
      :sources="sources"
      :sinks="sinks"
      :providers="providers"
      :presets="presets"
      :templates="templates"
      :filters="filters"
      :condition-descriptors="conditionDescriptors"
      :saving="savingProfile"
      :previewing="previewing"
      @save="saveProfile"
      @preview="previewDraft"
      @manage-templates="activeTab = 'templates'"
      @manage-filters="router.push({ name: 'filters' })"
    />

    <!-- 输出模板表单 -->
    <AIDigestTemplateForm v-model:show="showTemplateForm" :template="editingTemplate" @saved="loadTemplates" />

    <!-- 运行记录抽屉 -->
    <NDrawer v-model:show="showRunsDrawer" :width="720" placement="right">
      <NDrawerContent :title="`运行记录 · ${runsProfile?.name ?? ''}`" closable>
        <div class="drawer-toolbar">
          <NButton size="small" secondary @click="refreshRuns">刷新</NButton>
          <NButton size="small" quaternary type="error" @click="cleanupRuns">清理 30 天前记录</NButton>
        </div>
        <NDataTable
          :loading="runsLoading"
          :columns="runColumns"
          :data="runs"
          :bordered="false"
          :scroll-x="820"
          :row-key="(row: AIDigestRun) => row.id"
        >
          <template #empty>
            <div class="tab-empty"><p>暂无运行记录。</p></div>
          </template>
        </NDataTable>
      </NDrawerContent>
    </NDrawer>

    <!-- 运行详情 -->
    <NModal
      v-model:show="showDetail"
      preset="card"
      title="AI 运行详情"
      :style="{ width: 'min(1000px, calc(100vw - 32px))' }"
    >
      <AIDigestRunDetail :detail="selectedDetail" :loading="detailLoading" />
    </NModal>

    <!-- Provider 表单 -->
    <NModal
      v-model:show="showProviderForm"
      preset="card"
      :title="providerFormTitle"
      :style="{ width: 'min(760px, calc(100vw - 32px))' }"
    >
      <NForm label-placement="top" :show-feedback="false">
        <div class="provider-grid">
          <NFormItem label="名称">
            <NInput v-model:value="providerForm.name" placeholder="OpenAI / DeepSeek / Anthropic Gateway" />
          </NFormItem>
          <NFormItem label="接口类型">
            <NSelect v-model:value="providerForm.api_type" :options="apiTypeOptions" />
          </NFormItem>
          <NFormItem label="默认 Provider">
            <NSwitch v-model:value="providerForm.is_default" />
          </NFormItem>
          <NFormItem label="Base URL" class="span-2">
            <NInput v-model:value="providerForm.base_url" placeholder="https://api.openai.com/v1" />
          </NFormItem>
          <NFormItem label="默认模型">
            <NInput v-model:value="providerForm.model" placeholder="gpt-4o-mini" />
          </NFormItem>
          <NFormItem label="API Key" class="span-2">
            <NInput
              v-model:value="providerForm.api_key"
              type="password"
              show-password-on="click"
              :placeholder="editingProvider?.has_api_key ? '已配置，留空表示不修改' : 'sk-...'"
            />
          </NFormItem>
          <NFormItem label="Temperature">
            <NInputNumber v-model:value="providerForm.default_temperature" :min="0" :max="2" :step="0.1" class="full-input" />
          </NFormItem>
          <NFormItem label="超时秒数">
            <NInputNumber v-model:value="providerForm.timeout_seconds" :min="5" class="full-input" />
          </NFormItem>
          <NFormItem label="重试次数">
            <NInputNumber v-model:value="providerForm.max_retries" :min="0" class="full-input" />
          </NFormItem>
        </div>
      </NForm>
      <template #footer>
        <NSpace justify="space-between">
          <NButton secondary :loading="providerTesting" @click="() => testProvider()">测试当前配置</NButton>
          <NSpace>
            <NButton @click="showProviderForm = false">取消</NButton>
            <NButton type="primary" :loading="providerSaving" @click="saveProvider">
              {{ editingProviderId ? '保存' : '创建' }}
            </NButton>
          </NSpace>
        </NSpace>
      </template>
    </NModal>
  </NSpace>
</template>

<style scoped>
.tab-lead {
  margin: 2px 0 14px;
  color: var(--clay-text-3);
  font-size: 13px;
}

.drawer-toolbar {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-bottom: 12px;
}

.tab-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 28px 0;
  color: var(--clay-text-3);
}

.cell-stack {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.cell-title {
  font-weight: 700;
  color: var(--clay-text);
}

.provider-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(160px, 1fr));
  gap: 12px;
  align-items: start;
}

.provider-grid .span-2 {
  grid-column: span 2;
}

.full-input {
  width: 100%;
}

:deep(.row-actions) {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

@media (max-width: 820px) {
  .provider-grid {
    grid-template-columns: 1fr;
  }

  .provider-grid .span-2 {
    grid-column: auto;
  }
}
</style>
