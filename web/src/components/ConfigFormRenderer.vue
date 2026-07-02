<script setup lang="ts">
import { computed } from 'vue'
import type { SelectOption } from 'naive-ui'
import type { FieldSpec } from '@/types'

const model = defineModel<Record<string, unknown>>({ required: true })

const props = withDefaults(
  defineProps<{
    fields: FieldSpec[]
    disabled?: boolean
  }>(),
  {
    disabled: false,
  },
)

const visibleFields = computed(() => props.fields)

function fieldValue(field: FieldSpec): unknown {
  if (model.value[field.key] !== undefined) return model.value[field.key]
  return field.default
}

function updateField(field: FieldSpec, value: unknown): void {
  model.value = {
    ...model.value,
    [field.key]: normalizeValue(field, value),
  }
}

function normalizeValue(field: FieldSpec, value: unknown): unknown {
  if (field.type === 'number') {
    if (value === null || value === undefined || value === '') return undefined
    return Number(value)
  }
  if (field.type === 'string_list') {
    return Array.isArray(value) ? value.filter((item) => String(item).trim()) : []
  }
  if (field.type === 'key_value') {
    return parseKeyValue(String(value ?? ''))
  }
  return value
}

function textValue(field: FieldSpec): string {
  const value = fieldValue(field)
  if (field.type === 'key_value') return formatKeyValue(value)
  return typeof value === 'string' ? value : ''
}

function numberValue(field: FieldSpec): number | null {
  const value = fieldValue(field)
  return typeof value === 'number' ? value : null
}

function booleanValue(field: FieldSpec): boolean {
  return Boolean(fieldValue(field))
}

function stringListValue(field: FieldSpec): string[] {
  const value = fieldValue(field)
  if (Array.isArray(value)) return value.map(String)
  if (typeof value === 'string' && value) return [value]
  return []
}

function selectValue(field: FieldSpec): string | string[] | null {
  const value = fieldValue(field)
  if (Array.isArray(value)) return value.map(String)
  if (typeof value === 'string') return value
  return null
}

function selectOptions(field: FieldSpec): SelectOption[] {
  return (field.options ?? []).map((item) => ({ label: item.label, value: item.value }))
}

function formatKeyValue(value: unknown): string {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return ''
  return Object.entries(value as Record<string, unknown>)
    .map(([key, item]) => `${key}: ${String(item ?? '')}`)
    .join('\n')
}

function parseKeyValue(value: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const line of value.split(/\r?\n/)) {
    const text = line.trim()
    if (!text) continue
    const idx = text.includes(':') ? text.indexOf(':') : text.indexOf('=')
    if (idx <= 0) continue
    const key = text.slice(0, idx).trim()
    const item = text.slice(idx + 1).trim()
    if (key) out[key] = item
  }
  return out
}
</script>

<template>
  <div class="config-form">
    <NFormItem
      v-for="field in visibleFields"
      :key="field.key"
      :label="field.label"
      :required="field.required"
      :feedback="field.help"
    >
      <NInput
        v-if="field.type === 'text' || field.type === 'password'"
        :value="textValue(field)"
        :type="field.type === 'password' ? 'password' : 'text'"
        :disabled="disabled"
        :placeholder="field.placeholder"
        show-password-on="click"
        @update:value="(value: string) => updateField(field, value)"
      />
      <NInput
        v-else-if="field.type === 'textarea' || field.type === 'key_value'"
        :value="textValue(field)"
        type="textarea"
        :disabled="disabled"
        :placeholder="field.placeholder ?? (field.type === 'key_value' ? 'Header-Name: value' : '')"
        :autosize="{ minRows: field.type === 'key_value' ? 3 : 4 }"
        @update:value="(value: string) => updateField(field, value)"
      />
      <NInputNumber
        v-else-if="field.type === 'number'"
        :value="numberValue(field)"
        :disabled="disabled"
        :min="field.min"
        :max="field.max"
        :placeholder="field.placeholder"
        @update:value="(value: number | null) => updateField(field, value)"
      />
      <NSwitch
        v-else-if="field.type === 'boolean'"
        :value="booleanValue(field)"
        :disabled="disabled"
        @update:value="(value: boolean) => updateField(field, value)"
      />
      <NSelect
        v-else-if="field.type === 'select' || field.type === 'multi_select'"
        :value="selectValue(field)"
        :multiple="field.type === 'multi_select'"
        :disabled="disabled"
        :options="selectOptions(field)"
        :placeholder="field.placeholder"
        @update:value="(value: string | string[]) => updateField(field, value)"
      />
      <NDynamicTags
        v-else-if="field.type === 'string_list'"
        :value="stringListValue(field)"
        :disabled="disabled"
        @update:value="(value: string[]) => updateField(field, value)"
      />
    </NFormItem>
  </div>
</template>

<style scoped>
.config-form {
  display: grid;
  gap: 2px;
}
</style>
