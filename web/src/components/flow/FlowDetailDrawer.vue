<script setup lang="ts">
import type { FlowSourceNode } from '@/composables/useForwardingGraph'

const show = defineModel<boolean>('show', { required: true })

defineProps<{
  node?: FlowSourceNode | null
}>()

const emit = defineEmits<{
  editSource: [id: number]
  editRule: [id: number]
  editSink: [id: number]
}>()
</script>

<template>
  <NDrawer v-model:show="show" :width="460" placement="right">
    <NDrawerContent :title="node?.source.name ?? '关系详情'" :native-scrollbar="false">
      <template v-if="node">
        <NSpace vertical size="large">
          <section class="detail-section">
            <div class="section-title">监听来源</div>
            <NDescriptions :column="1" size="small" bordered>
              <NDescriptionsItem label="来源">{{ node.source.name }}</NDescriptionsItem>
              <NDescriptionsItem label="账号">{{ node.account?.name ?? `#${node.source.account_id}` }}</NDescriptionsItem>
              <NDescriptionsItem label="Peer ID">{{ node.source.peer_id }}</NDescriptionsItem>
              <NDescriptionsItem label="状态">{{ node.source.enabled ? '启用' : '停用' }}</NDescriptionsItem>
            </NDescriptions>
          </section>

          <NAlert v-if="node.warnings.length" type="warning" title="需要处理">
            <ul class="warn-list">
              <li v-for="warning in node.warnings" :key="warning">{{ warning }}</li>
            </ul>
          </NAlert>

          <section class="detail-section">
            <div class="section-title">规则链路</div>
            <div v-if="node.rules.length" class="rule-list">
              <article v-for="item in node.rules" :key="item.rule.id" class="rule-card">
                <div class="rule-head">
                  <div>
                    <div class="rule-name">{{ item.rule.name }}</div>
                    <div class="rule-meta">
                      优先级 {{ item.rule.priority }} · {{ item.rule.enabled ? '启用' : '停用' }}
                    </div>
                  </div>
                  <NButton size="small" text type="primary" @click="emit('editRule', item.rule.id)">
                    编辑
                  </NButton>
                </div>
                <div class="target-list">
                  <div v-if="!item.targets.length" class="empty-line">没有目标渠道</div>
                  <div v-for="target in item.targets" :key="`${item.rule.id}-${target.sinkId}-${target.templateId ?? 0}`" class="target-line">
                    <span>{{ target.sink?.name ?? `渠道 #${target.sinkId}` }}</span>
                    <span class="muted">{{ target.template?.name ?? '原文' }}</span>
                  </div>
                </div>
              </article>
            </div>
            <NEmpty v-else description="这个来源还没有进入任何规则" />
          </section>
        </NSpace>
      </template>

      <template #footer>
        <NSpace justify="end">
          <NButton v-if="node" @click="emit('editSource', node.source.id)">查看来源</NButton>
          <NButton type="primary" @click="show = false">关闭</NButton>
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

.rule-list {
  display: grid;
  gap: 10px;
}

.rule-card {
  border: 1px solid var(--clay-border);
  border-radius: 8px;
  padding: 12px;
  background: var(--clay-surface);
}

.rule-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.rule-name {
  font-weight: 700;
}

.rule-meta,
.muted,
.empty-line {
  color: var(--clay-text-3);
  font-size: 12px;
}

.target-list {
  display: grid;
  gap: 6px;
  margin-top: 10px;
}

.target-line {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
  padding: 7px 9px;
  border-radius: 7px;
  background: var(--clay-surface-2);
  font-size: 13px;
}
</style>
