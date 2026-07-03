<script setup lang="ts">
/**
 * 页头黏土条：图标 + 标题 + 描述 + 右侧操作区。
 * 窄屏时操作区自动换行并撑满，配合各列表页的工具栏统一观感。
 */
import ClayIcon from '@/components/ClayIcon.vue'

defineProps<{ title: string; desc?: string; icon?: string }>()
</script>

<template>
  <header class="page-header">
    <div class="ph-heading">
      <div v-if="icon" class="ph-icon">
        <ClayIcon :name="icon" :size="22" />
      </div>
      <div class="ph-titles">
        <h2 class="ph-title">{{ title }}</h2>
        <p v-if="desc" class="ph-desc">{{ desc }}</p>
      </div>
    </div>
    <div v-if="$slots.actions" class="ph-actions">
      <slot name="actions" />
    </div>
  </header>
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  padding: 16px 20px;
  border-radius: 22px;
  background: var(--clay-surface);
  box-shadow: var(--clay-out-sm), var(--clay-inset-hi);
}
.ph-heading {
  display: flex;
  align-items: center;
  gap: 14px;
  min-width: 0;
}
.ph-icon {
  width: 46px;
  height: 46px;
  border-radius: 15px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  color: #2f8fd6;
  background: rgba(58, 160, 227, 0.14);
  box-shadow:
    inset 2px 2px 5px rgba(255, 255, 255, 0.55),
    inset -3px -3px 6px rgba(56, 104, 150, 0.12);
}
.ph-titles {
  min-width: 0;
}
.ph-title {
  margin: 0;
  font-size: 19px;
  font-weight: 900;
  letter-spacing: 0.2px;
  line-height: 1.2;
  color: var(--n-text-color, #22364a);
}
.ph-desc {
  margin: 3px 0 0;
  font-size: 13px;
  font-weight: 600;
  color: #8aa0b4;
}
.ph-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

@media (max-width: 640px) {
  .page-header {
    padding: 14px 16px;
  }
  .ph-actions {
    width: 100%;
  }
  /* 操作按钮在窄屏平分一行，更好点按 */
  .ph-actions :deep(.n-button) {
    flex: 1;
  }
}
</style>
