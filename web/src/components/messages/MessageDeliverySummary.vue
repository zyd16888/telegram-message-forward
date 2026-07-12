<script setup lang="ts">
import { computed } from 'vue'
import type { MessageDeliveryCounts } from '@/types'

const props = defineProps<{ counts: MessageDeliveryCounts }>()
const items = computed(() => [
  { key: 'success', label: '成功', value: props.counts.success, type: 'success' as const },
  { key: 'active', label: '进行中', value: props.counts.pending + props.counts.processing + props.counts.retrying, type: 'warning' as const },
  { key: 'failed', label: '失败', value: props.counts.failed + props.counts.dead, type: 'error' as const },
].filter((item) => item.value > 0))
</script>

<template>
  <div class="summary">
    <NTag v-if="counts.total === 0" size="small" :bordered="false">未产生投递</NTag>
    <NTag v-for="item in items" :key="item.key" size="small" :type="item.type" :bordered="false">
      {{ item.label }} {{ item.value }}
    </NTag>
  </div>
</template>

<style scoped>
.summary { display: flex; flex-wrap: wrap; gap: 6px; }
</style>
