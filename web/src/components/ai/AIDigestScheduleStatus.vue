<script setup lang="ts">
import { computed } from 'vue'
import type { AIDigestSchedule } from '@/types'
import { formatDateTime, resolvedTimeZone } from '@/utils/datetime'

const props = defineProps<{
  schedule: AIDigestSchedule
  enabled: boolean
  now: number
}>()

const scheduleLabel = computed(() => {
  const schedule = props.schedule
  if (schedule.type === 'interval') return `每 ${schedule.interval_minutes || 60} 分钟`
  if (schedule.type === 'daily') return `每日 ${schedule.time || '09:00'}`
  if (schedule.type === 'cron') return `Cron ${schedule.cron || ''}`
  return '仅手动'
})

const nextTimestamp = computed(() => {
  if (!props.enabled || props.schedule.type === 'manual' || !props.schedule.next_run_at) return null
  const value = Date.parse(props.schedule.next_run_at)
  return Number.isNaN(value) ? null : value
})

const nextRunLabel = computed(() => {
  if (nextTimestamp.value === null) return '下次执行 --'
  return `下次执行 ${formatDateTime(props.schedule.next_run_at, { timeZone: props.schedule.timezone, seconds: false })}`
})

const timeZoneLabel = computed(() => resolvedTimeZone(props.schedule.timezone))

const relativeLabel = computed(() => {
  if (nextTimestamp.value === null) return ''
  const diff = nextTimestamp.value - props.now
  if (diff <= 0) return '等待调度'
  const minutes = Math.ceil(diff / 60_000)
  if (minutes < 60) return `${minutes} 分钟后`
  const hours = Math.ceil(minutes / 60)
  if (hours < 24) return `${hours} 小时后`
  return `${Math.ceil(hours / 24)} 天后`
})

const isDue = computed(() => nextTimestamp.value !== null && nextTimestamp.value <= props.now)
</script>

<template>
  <div class="schedule-status">
    <span class="schedule-label">{{ scheduleLabel }}</span>
    <span class="next-run" :class="{ due: isDue }">
      {{ nextRunLabel }}<template v-if="relativeLabel"> · {{ relativeLabel }}</template>
    </span>
    <span v-if="nextTimestamp !== null" class="timezone-label">{{ timeZoneLabel }}</span>
  </div>
</template>

<style scoped>
.schedule-status {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 190px;
}

.schedule-label {
  color: var(--clay-text);
  font-weight: 600;
}

.next-run {
  color: var(--clay-text-3);
  font-size: 12px;
  line-height: 1.4;
}

.timezone-label {
  color: var(--clay-text-3);
  font-size: 11px;
}

.next-run.due {
  color: var(--clay-warning, #b26a00);
  font-weight: 600;
}
</style>
