<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { useDialog, useMessage } from 'naive-ui'
import { useRouter } from 'vue-router'
import { aiApi, filtersApi, flowsApi, sinksApi, sourcesApi } from '@/api/client'
import type {
  AIDigestProfile,
  AIDigestProfileRequest,
  AIDigestPreset,
  AIDigestOutputTemplate,
  AIDigestRunDetail as AIDigestRunDetailType,
  AIProvider,
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
import AIDigestRunsDrawer from '@/components/ai/AIDigestRunsDrawer.vue'
import AIDigestProvidersPanel from '@/components/ai/AIDigestProvidersPanel.vue'
import AIDigestProfilesTable from '@/components/ai/AIDigestProfilesTable.vue'
import AIDigestTemplatesTable from '@/components/ai/AIDigestTemplatesTable.vue'

const message = useMessage()
const dialog = useDialog()
const router = useRouter()

const activeTab = ref<'profiles' | 'templates' | 'providers'>('profiles')

const loading = ref(false)
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

const runsProfile = ref<AIDigestProfile | null>(null)
const showRunsDrawer = ref(false)

const selectedDetail = ref<AIDigestRunDetailType | null>(null)
const showDetail = ref(false)
const detailLoading = ref(false)
const detailTimeZone = ref<string>()

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
  detailTimeZone.value = payload.schedule.timezone
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
  detailTimeZone.value = profile.schedule.timezone
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
  detailTimeZone.value = profile.schedule.timezone
  try {
    selectedDetail.value = await aiApi.profiles.run(profile.id)
    await loadAll()
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
function openRunsDrawer(profile: AIDigestProfile): void {
  runsProfile.value = profile
  showRunsDrawer.value = true
}

async function openRunDetail(id: number, timeZone?: string): Promise<void> {
  detailLoading.value = true
  selectedDetail.value = null
  showDetail.value = true
  detailTimeZone.value = timeZone
  try {
    selectedDetail.value = await aiApi.runs.get(id)
  } catch (e) {
    message.error('加载运行详情失败：' + errText(e))
  } finally {
    detailLoading.value = false
  }
}

async function openClonedProfile(profile: AIDigestProfile): Promise<void> {
  await loadAll()
  selectedProfile.value = profiles.value.find((item) => item.id === profile.id) ?? profile
  showForm.value = true
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
        <NButton secondary @click="loadAll">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </NButton>
      </template>
    </PageHeader>

    <NTabs v-model:value="activeTab" type="segment" size="large" animated>
      <NTabPane name="profiles" tab="整理任务">
        <AIDigestProfilesTable
          :profiles="profiles"
          :templates="templates"
          :loading="loading"
          :now="scheduleNow"
          @create="newProfile"
          @run="runProfile"
          @preview="previewProfile"
          @records="openRunsDrawer"
          @edit="editProfile"
          @remove="confirmRemoveProfile"
        />
      </NTabPane>

      <NTabPane name="templates" tab="输出模板">
        <AIDigestTemplatesTable
          :templates="templates"
          :profiles="profiles"
          :loading="loading"
          @create="newTemplate"
          @edit="editTemplate"
          @remove="confirmRemoveTemplate"
        />
      </NTabPane>

      <NTabPane name="providers" tab="AI 服务">
        <AIDigestProvidersPanel :providers="providers" :loading="loading" @changed="loadProviders" />
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

    <AIDigestRunsDrawer
      v-model:show="showRunsDrawer"
      :profile="runsProfile"
      @open-detail="openRunDetail"
      @profile-cloned="openClonedProfile"
    />

    <!-- 运行详情 -->
    <NModal
      v-model:show="showDetail"
      preset="card"
      title="AI 运行详情"
      :style="{ width: 'min(1000px, calc(100vw - 32px))' }"
    >
      <AIDigestRunDetail :detail="selectedDetail" :loading="detailLoading" :time-zone="detailTimeZone" />
    </NModal>
  </NSpace>
</template>
