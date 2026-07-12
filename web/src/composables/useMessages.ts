import { computed, onBeforeUnmount, onMounted, shallowRef, type Ref } from 'vue'
import { messagesApi } from '@/api/client'
import type { MessageDetail, MessageItem } from '@/types'

export interface MessageFilters {
  keyword: string
  sourceId: number | null
  sourceType: string
  messageType: string
  deliveryStatus: string
  media: '' | 'true' | 'false'
  hours: number
}

export function defaultMessageFilters(): MessageFilters {
  return { keyword: '', sourceId: null, sourceType: '', messageType: '', deliveryStatus: '', media: '', hours: 24 }
}

export function useMessages(filters: Ref<MessageFilters>) {
  const items = shallowRef<MessageItem[]>([])
  const detail = shallowRef<MessageDetail | null>(null)
  const loading = shallowRef(false)
  const loadingMore = shallowRef(false)
  const detailLoading = shallowRef(false)
  const hasMore = shallowRef(false)
  const nextCursor = shallowRef(0)
  const hasNewMessages = shallowRef(false)

  const latestID = computed(() => items.value[0]?.id ?? 0)
  let timer: number | undefined

  function queryParams(beforeID = 0): Record<string, unknown> {
    const value = filters.value
    const params: Record<string, unknown> = { limit: 30 }
    if (value.keyword.trim()) params.q = value.keyword.trim()
    if (value.sourceId) params.source_id = value.sourceId
    if (value.sourceType) params.source_type = value.sourceType
    if (value.messageType) params.message_type = value.messageType
    if (value.deliveryStatus) params.delivery_status = value.deliveryStatus
    if (value.media) params.has_media = value.media
    if (value.hours > 0) params.from = new Date(Date.now() - value.hours * 3600_000).toISOString()
    if (beforeID > 0) params.before_id = beforeID
    return params
  }

  async function load(reset = true) {
    if (reset) loading.value = true
    else loadingMore.value = true
    try {
      const page = await messagesApi.page(queryParams(reset ? 0 : nextCursor.value))
      items.value = reset ? page.data : [...items.value, ...page.data]
      hasMore.value = page.has_more
      nextCursor.value = page.next_cursor
      if (reset) hasNewMessages.value = false
    } finally {
      loading.value = false
      loadingMore.value = false
    }
  }

  async function checkNew() {
    if (!latestID.value || document.visibilityState !== 'visible') return
    try {
      const page = await messagesApi.page({ ...queryParams(), limit: 1 })
      hasNewMessages.value = (page.data[0]?.id ?? 0) > latestID.value
    } catch {
      // 后台轻量探测失败不打断当前列表。
    }
  }

  async function openDetail(id: number) {
    detailLoading.value = true
    detail.value = null
    try {
      detail.value = await messagesApi.get(id)
    } finally {
      detailLoading.value = false
    }
  }

  function clearDetail() {
    detail.value = null
  }

  onMounted(() => {
    timer = window.setInterval(() => void checkNew(), 5000)
  })
  onBeforeUnmount(() => window.clearInterval(timer))

  return {
    items, detail, loading, loadingMore, detailLoading, hasMore, hasNewMessages,
    load, loadMore: () => load(false), openDetail, clearDetail,
  }
}
