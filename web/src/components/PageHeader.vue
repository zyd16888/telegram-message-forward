<script setup lang="ts">
import ClayIcon from '@/components/ClayIcon.vue'

defineProps<{ title: string; desc?: string; icon?: string }>()
</script>

<template>
  <header class="page-header">
    <div class="ph-heading">
      <div v-if="icon" class="ph-icon">
        <ClayIcon :name="icon" :size="20" />
      </div>
      <div class="ph-titles">
        <h1 class="ph-title">{{ title }}</h1>
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
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  padding: 2px 0 4px;
}

.ph-heading {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-width: 0;
}

.ph-icon {
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
  border: 0;
  border-radius: 8px;
  color: var(--clay-primary);
  background: var(--clay-primary-soft);
  border: 1px solid color-mix(in srgb, var(--clay-primary) 16%, var(--clay-border));
}

.ph-titles {
  min-width: 0;
}

.ph-title {
  margin: 0;
  color: var(--clay-text);
  font-size: 20px;
  font-weight: 750;
  line-height: 1.25;
  text-wrap: balance;
}

.ph-desc {
  margin: 4px 0 0;
  color: var(--clay-text-2);
  font-size: 13px;
}

.ph-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

@media (max-width: 640px) {
  .ph-actions {
    width: 100%;
  }

  .ph-actions :deep(.n-button) {
    flex: 1;
  }
}
</style>
