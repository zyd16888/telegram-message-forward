<script setup lang="ts">
const keyword = defineModel<string>('keyword', { required: true })
const onlyWarnings = defineModel<boolean>('onlyWarnings', { required: true })
const showUnusedNodes = defineModel<boolean>('showUnusedNodes', { required: true })
const showResourceLayer = defineModel<boolean>('showResourceLayer', { required: true })
const templateFilterId = defineModel<number | null>('templateFilterId', { required: true })

defineProps<{
  hiddenResourceCount: number
  templateFilterName: string
  stats: {
    linkedSources: number
    totalSources: number
    enabledRules: number
    totalRules: number
    enabledSinks: number
    totalSinks: number
    warningRules: number
  }
}>()
</script>

<template>
  <div class="board-toolbar">
    <div class="toolbar-filters">
      <NInput v-model:value="keyword" clearable class="search-input" placeholder="搜索来源、Flow、渠道" />
      <NCheckbox v-model:checked="onlyWarnings">只看异常 Flow</NCheckbox>
      <NCheckbox v-model:checked="showUnusedNodes">
        显示未接入资源
        <template v-if="hiddenResourceCount">（{{ hiddenResourceCount }}）</template>
      </NCheckbox>
      <NCheckbox v-model:checked="showResourceLayer">显示资源层</NCheckbox>
      <NTag v-if="templateFilterId" closable size="small" type="info" @close="templateFilterId = null">
        模板：{{ templateFilterName }}
      </NTag>
    </div>
    <div class="toolbar-stats">
      <span>来源 {{ stats.linkedSources }}/{{ stats.totalSources }} 已接入</span>
      <span>Flow {{ stats.enabledRules }}/{{ stats.totalRules }} 启用</span>
      <span>渠道 {{ stats.enabledSinks }}/{{ stats.totalSinks }} 启用</span>
      <span :class="{ 'stat-warn': stats.warningRules > 0 }">异常 {{ stats.warningRules }}</span>
    </div>
  </div>
</template>

<style scoped>
.board-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
  padding: 10px 12px;
  border-radius: 14px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm);
}

.toolbar-filters {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.search-input {
  width: 260px;
}

.toolbar-stats {
  display: flex;
  align-items: center;
  gap: 14px;
  color: var(--clay-text-2);
  font-size: 13px;
  flex-wrap: wrap;
}

.toolbar-stats span {
  padding: 4px 9px;
  border: 1px solid var(--clay-border);
  border-radius: 999px;
  background: var(--clay-surface-2);
}

.stat-warn {
  color: #b45309;
  font-weight: 700;
}

@media (max-width: 680px) {
  .search-input {
    width: 100%;
  }
}
</style>
