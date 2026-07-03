<script setup lang="ts">
import type { FlowRuleGraphNode } from '@/composables/useForwardingGraph'

const show = defineModel<boolean>('show', { required: true })

defineProps<{
  node?: FlowRuleGraphNode | null
}>()

const emit = defineEmits<{
  editRule: [id: number]
  editSource: [id: number]
  editSink: [id: number]
  createRule: []
}>()
</script>

<template>
  <NDrawer v-model:show="show" :width="500" placement="right">
    <NDrawerContent :title="node?.rule.name ?? '规则详情'" :native-scrollbar="false">
      <template v-if="node">
        <NSpace vertical size="large">
          <section class="detail-section">
            <div class="section-title">规则</div>
            <NDescriptions :column="1" size="small" bordered>
              <NDescriptionsItem label="名称">{{ node.rule.name }}</NDescriptionsItem>
              <NDescriptionsItem label="优先级">{{ node.rule.priority }}</NDescriptionsItem>
              <NDescriptionsItem label="状态">{{ node.rule.enabled ? '启用' : '停用' }}</NDescriptionsItem>
              <NDescriptionsItem label="命中即停">{{ node.rule.stop_on_match ? '是' : '否' }}</NDescriptionsItem>
            </NDescriptions>
          </section>

          <NAlert v-if="node.warnings.length" type="warning" title="需要处理">
            <ul class="warn-list">
              <li v-for="warning in node.warnings" :key="warning">{{ warning }}</li>
            </ul>
          </NAlert>

          <section class="detail-section">
            <div class="section-title">来源</div>
            <div v-if="node.sources.length" class="line-list">
              <button
                v-for="source in node.sources"
                :key="source.sourceId"
                type="button"
                class="line-item"
                @click="source.source && emit('editSource', source.source.id)"
              >
                <span>{{ source.source?.name ?? `来源 #${source.sourceId}` }}</span>
                <span class="muted">{{ source.account?.name ?? '账号未知' }}</span>
              </button>
            </div>
            <NEmpty v-else size="small" description="未指定来源" />
          </section>

          <section class="detail-section">
            <div class="section-title">匹配条件</div>
            <div v-if="node.rule.conditions.length" class="line-list">
              <div v-for="(condition, index) in node.rule.conditions" :key="`${condition.type}-${index}`" class="line-item static">
                <span>{{ condition.type }}</span>
              </div>
            </div>
            <NEmpty v-else size="small" description="无条件，来源消息直接匹配" />
          </section>

          <section class="detail-section">
            <div class="section-title">处理器</div>
            <div v-if="node.rule.processors.length" class="line-list">
              <div v-for="(processor, index) in node.rule.processors" :key="`${processor.type}-${index}`" class="line-item static">
                <span>{{ processor.type }}</span>
              </div>
            </div>
            <NEmpty v-else size="small" description="不做额外处理" />
          </section>

          <section class="detail-section">
            <div class="section-title">目标</div>
            <div v-if="node.targets.length" class="line-list">
              <button
                v-for="target in node.targets"
                :key="`${target.sinkId}-${target.templateId ?? 0}`"
                type="button"
                class="line-item"
                @click="target.sink && emit('editSink', target.sink.id)"
              >
                <span>{{ target.sink?.name ?? `渠道 #${target.sinkId}` }}</span>
                <span class="muted">{{ target.template?.name ?? '原文' }}</span>
              </button>
            </div>
            <NEmpty v-else size="small" description="未配置目标渠道" />
          </section>
        </NSpace>
      </template>

      <template #footer>
        <NSpace justify="end">
          <NButton v-if="node" @click="emit('editRule', node.rule.id)">编辑规则</NButton>
          <NButton type="primary" @click="emit('createRule')">新建规则</NButton>
          <NButton @click="show = false">关闭</NButton>
        </NSpace>
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.detail-section {
  display: grid;
  gap: 10px;
}

.section-title {
  color: var(--clay-text);
  font-size: 14px;
  font-weight: 800;
}

.warn-list {
  margin: 0;
  padding-left: 18px;
}

.line-list {
  display: grid;
  gap: 8px;
}

.line-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  min-width: 0;
  padding: 9px 10px;
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  color: var(--clay-text);
  background: var(--clay-surface);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.line-item.static {
  cursor: default;
}

.line-item:not(.static):hover,
.line-item:not(.static):focus-visible {
  border-color: var(--clay-border-strong);
  outline: none;
}

.muted {
  color: var(--clay-text-3);
  font-size: 12px;
  white-space: nowrap;
}
</style>
