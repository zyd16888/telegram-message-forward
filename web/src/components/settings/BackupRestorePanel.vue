<script setup lang="ts">
import { computed, ref, shallowRef } from 'vue'
import { useDialog, useMessage } from 'naive-ui'
import { settingsApi } from '@/api/client'
import type { BackupPreview } from '@/types'
import { errText } from '@/utils/error'

const message = useMessage()
const dialog = useDialog()

const exportPassword = ref('')
const exportPasswordConfirm = ref('')
const includeSessions = shallowRef(true)
const exporting = shallowRef(false)
const restoreFile = shallowRef<File | null>(null)
const restorePassword = ref('')
const preview = shallowRef<BackupPreview | null>(null)
const inspecting = shallowRef(false)
const restoring = shallowRef(false)

const countLabels: Record<string, string> = {
  telegram_apps: 'Telegram App', proxies: '代理', accounts: 'TG 账号', sources: '监听源', sinks: '目标渠道',
  templates: '渲染模板', filters: '过滤器', flows: 'Flow', settings: '系统设置',
  ai_profiles: 'AI Profile', ai_output_templates: 'AI 输出模板',
}

const countItems = computed(() => Object.entries(preview.value?.manifest.counts ?? {}).filter(([, value]) => value > 0))

async function exportBackup() {
  if (exportPassword.value.length < 8) { message.warning('备份口令至少需要 8 个字符'); return }
  if (exportPassword.value !== exportPasswordConfirm.value) { message.warning('两次输入的备份口令不一致'); return }
  exporting.value = true
  try {
    const response = await settingsApi.backups.export(exportPassword.value, includeSessions.value)
    const disposition = String(response.headers['content-disposition'] ?? '')
    const filename = disposition.match(/filename="?([^";]+)"?/i)?.[1] ?? 'tmf-config.tmfbackup'
    const url = URL.createObjectURL(response.data)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    link.click()
    URL.revokeObjectURL(url)
    exportPassword.value = ''
    exportPasswordConfirm.value = ''
    message.success('配置备份已生成')
  } catch (e) { message.error('导出失败：' + await backupErrorText(e)) } finally { exporting.value = false }
}

async function backupErrorText(error: unknown): Promise<string> {
  const blob = (error as { response?: { data?: unknown } })?.response?.data
  if (blob instanceof Blob) {
    try {
      const parsed = JSON.parse(await blob.text()) as { error?: string }
      if (parsed.error) return parsed.error
    } catch { /* 使用通用错误文本。 */ }
  }
  return errText(error)
}

function selectFile(event: Event) {
  const input = event.target as HTMLInputElement
  restoreFile.value = input.files?.[0] ?? null
  preview.value = null
}

async function inspectBackup() {
  if (!restoreFile.value) { message.warning('请选择备份文件'); return }
  if (restorePassword.value.length < 8) { message.warning('请输入备份口令'); return }
  inspecting.value = true
  try { preview.value = await settingsApi.backups.inspect(restoreFile.value, restorePassword.value) }
  catch (e) { preview.value = null; message.error('解析失败：' + errText(e)) }
  finally { inspecting.value = false }
}

function confirmRestore() {
  if (!restoreFile.value || !preview.value?.can_restore) return
  dialog.warning({
    title: '恢复配置',
    content: '恢复会更新或补回备份中的配置，完成后需要重启服务。备份之外的现有对象不会自动删除。确认继续？',
    positiveText: '确认恢复', negativeText: '取消',
    onPositiveClick: restoreBackup,
  })
}

async function restoreBackup() {
  if (!restoreFile.value) return false
  restoring.value = true
  try {
    const result = await settingsApi.backups.restore(restoreFile.value, restorePassword.value)
    message.success(result.restart_required ? '配置恢复完成，请重启服务使全部运行时配置生效' : '配置恢复完成')
    preview.value = null
    restoreFile.value = null
    restorePassword.value = ''
    return true
  } catch (e) { message.error('恢复失败：' + errText(e)); return false }
  finally { restoring.value = false }
}
</script>

<template>
  <NCard title="配置备份与恢复">
    <div class="backup-grid">
      <section class="backup-section">
        <header><h3>导出配置</h3><p>生成口令加密的可迁移配置包，不包含消息、投递记录、管理员和媒体文件。</p></header>
        <NForm :show-feedback="false" label-placement="top">
          <NFormItem label="备份口令">
            <NInput v-model:value="exportPassword" type="password" show-password-on="click" placeholder="至少 8 个字符，请妥善保存" />
          </NFormItem>
          <NFormItem label="确认口令">
            <NInput v-model:value="exportPasswordConfirm" type="password" show-password-on="click" placeholder="再次输入备份口令" @keyup.enter="exportBackup" />
          </NFormItem>
          <NFormItem label="Telegram 登录状态">
            <div class="switch-line"><NSwitch v-model:value="includeSessions" /><span>包含已登录账号 session，恢复后无需重新登录</span></div>
          </NFormItem>
        </NForm>
        <NButton type="primary" :loading="exporting" @click="exportBackup">导出加密备份</NButton>
      </section>

      <section class="backup-section">
        <header><h3>恢复配置</h3><p>先解析并预览影响范围，确认后才会在单个数据库事务中恢复。</p></header>
        <label class="file-picker">
          <input type="file" accept=".tmfbackup,application/json" @change="selectFile" />
          <span>{{ restoreFile?.name || '选择 .tmfbackup 文件' }}</span>
        </label>
        <NInput v-model:value="restorePassword" type="password" show-password-on="click" placeholder="输入备份口令" @keyup.enter="inspectBackup" />
        <NButton secondary :loading="inspecting" @click="inspectBackup">解析并预览</NButton>

        <div v-if="preview" class="preview" :class="{ blocked: !preview.can_restore }">
          <div class="preview-head">
            <strong>{{ preview.can_restore ? '可以恢复' : '不能恢复' }}</strong>
            <NTag :type="preview.can_restore ? 'success' : 'error'" size="small">格式 v{{ preview.manifest.version }}</NTag>
          </div>
          <p>{{ preview.warning || (preview.target_empty ? '目标实例为空，将重建配置引用' : '同一实例，将更新或补回备份对象') }}</p>
          <div class="counts">
            <span v-for="([key, value]) in countItems" :key="key">{{ countLabels[key] || key }} <strong>{{ value }}</strong></span>
          </div>
          <NButton v-if="preview.can_restore" type="warning" :loading="restoring" @click="confirmRestore">恢复这些配置</NButton>
        </div>
      </section>
    </div>
  </NCard>
</template>

<style scoped>
.backup-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 24px; }
.backup-section { display: flex; flex-direction: column; gap: 14px; min-width: 0; }
.backup-section + .backup-section { padding-left: 24px; border-left: 1px solid var(--clay-border); }
header h3 { margin: 0; font-size: 15px; }
header p, .preview p { margin: 5px 0 0; color: var(--clay-text-2); font-size: 13px; line-height: 1.55; }
.switch-line { display: flex; align-items: center; gap: 10px; color: var(--clay-text-2); font-size: 13px; }
.file-picker { min-height: 42px; padding: 0 12px; border: 1px dashed var(--clay-border-strong); border-radius: 8px; display: flex; align-items: center; color: var(--clay-text-2); background: var(--clay-surface-2); cursor: pointer; }
.file-picker:hover { border-color: var(--clay-primary); color: var(--clay-primary); }
.file-picker:focus-within { outline: 2px solid var(--clay-primary); outline-offset: 2px; }
.file-picker input { position: absolute; width: 1px; height: 1px; opacity: 0; }
.file-picker span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.preview { padding: 14px; border: 1px solid color-mix(in srgb, #2fb896 40%, var(--clay-border)); border-radius: 8px; background: color-mix(in srgb, #2fb896 7%, var(--clay-surface)); }
.preview.blocked { border-color: color-mix(in srgb, #e55c54 40%, var(--clay-border)); background: color-mix(in srgb, #e55c54 7%, var(--clay-surface)); }
.preview-head { display: flex; justify-content: space-between; gap: 10px; }
.counts { display: flex; flex-wrap: wrap; gap: 7px; margin: 12px 0; }
.counts span { padding: 5px 8px; border: 1px solid var(--clay-border); border-radius: 6px; background: var(--clay-surface); font-size: 12px; }
@media (max-width: 850px) { .backup-grid { grid-template-columns: 1fr; } .backup-section + .backup-section { padding: 24px 0 0; border-left: 0; border-top: 1px solid var(--clay-border); } }
</style>
