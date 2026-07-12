<script setup lang="ts">
import { onMounted, ref, shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import { useMessage } from 'naive-ui'
import { sourcesApi } from '@/api/client'
import type { Source } from '@/types'
import { errText } from '@/utils/error'
import { defaultMessageFilters, useMessages } from '@/composables/useMessages'
import PageHeader from '@/components/PageHeader.vue'
import ClayIcon from '@/components/ClayIcon.vue'
import MessageFilters from '@/components/messages/MessageFilters.vue'
import MessageList from '@/components/messages/MessageList.vue'
import MessageDetailDrawer from '@/components/messages/MessageDetailDrawer.vue'

const router = useRouter()
const message = useMessage()
const filters = ref(defaultMessageFilters())
const sources = shallowRef<Source[]>([])
const detailShow = shallowRef(false)
const state = useMessages(filters)

async function search() {
  try { await state.load() } catch (e) { message.error('加载消息失败：' + errText(e)) }
}

async function reset() {
  filters.value = defaultMessageFilters()
  await search()
}

async function openDetail(id: number) {
  detailShow.value = true
  try { await state.openDetail(id) } catch (e) { message.error('加载详情失败：' + errText(e)) }
}

async function loadMore() {
  try { await state.loadMore() } catch (e) { message.error('加载更多失败：' + errText(e)) }
}

function openDelivery(id: number) {
  void router.push({ name: 'deliveries', query: { task_id: String(id) } })
}

onMounted(async () => {
  void search()
  try { sources.value = await sourcesApi.list() } catch { /* 来源下拉为空不阻断消息列表。 */ }
})
</script>

<template>
  <div class="messages-page">
    <PageHeader title="消息中心" desc="查看系统实时接收并已入库的消息及其投递结果" icon="messages">
      <template #actions>
        <NButton secondary :loading="state.loading.value" @click="search">
          <template #icon><ClayIcon name="refresh" :size="16" /></template>
          刷新
        </NButton>
      </template>
    </PageHeader>

    <button v-if="state.hasNewMessages.value" type="button" class="new-message-bar" @click="search">
      有新消息，点击刷新列表
    </button>

    <MessageFilters v-model:filters="filters" :sources="sources" @search="search" @reset="reset" />
    <MessageList
      :items="state.items.value"
      :loading="state.loading.value"
      :loading-more="state.loadingMore.value"
      :has-more="state.hasMore.value"
      @select="openDetail"
      @more="loadMore"
    />
    <MessageDetailDrawer
      v-model:show="detailShow"
      :detail="state.detail.value"
      :loading="state.detailLoading.value"
      @delivery="openDelivery"
    />
  </div>
</template>

<style scoped>
.messages-page { display: grid; gap: 16px; }
.new-message-bar { width: 100%; min-height: 38px; border: 1px solid color-mix(in srgb, var(--clay-primary) 35%, var(--clay-border)); border-radius: 8px; color: var(--clay-primary); background: var(--clay-primary-soft); font-weight: 700; cursor: pointer; }
.new-message-bar:hover { border-color: var(--clay-primary); }
</style>
