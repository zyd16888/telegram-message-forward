import { computed, shallowRef } from 'vue'
import { accountsApi, rulesApi, sinksApi, sourcesApi, templatesApi } from '@/api/client'
import type { Account, Rule, Sink, Source, Template } from '@/types'

export interface FlowTargetNode {
  sink: Sink | null
  template: Template | null
  sinkId: number
  templateId?: number
}

export interface FlowRuleNode {
  rule: Rule
  targets: FlowTargetNode[]
}

export interface FlowSourceNode {
  source: Source
  account: Account | null
  rules: FlowRuleNode[]
  warnings: string[]
}

export function useForwardingGraph() {
  const accounts = shallowRef<Account[]>([])
  const sources = shallowRef<Source[]>([])
  const rules = shallowRef<Rule[]>([])
  const sinks = shallowRef<Sink[]>([])
  const templates = shallowRef<Template[]>([])
  const loading = shallowRef(false)

  const accountMap = computed(() => new Map(accounts.value.map((item) => [item.id, item])))
  const sinkMap = computed(() => new Map(sinks.value.map((item) => [item.id, item])))
  const templateMap = computed(() => new Map(templates.value.map((item) => [item.id, item])))

  const sourceNodes = computed<FlowSourceNode[]>(() =>
    sources.value.map((source) => {
      const matchedRules = rules.value
        .filter((rule) => rule.source_ids.includes(source.id))
        .sort((a, b) => b.priority - a.priority)
        .map((rule) => ({
          rule,
          targets: rule.targets.map((target) => ({
            sinkId: target.sink_id,
            templateId: target.template_id,
            sink: sinkMap.value.get(target.sink_id) ?? null,
            template: target.template_id ? templateMap.value.get(target.template_id) ?? null : null,
          })),
        }))

      return {
        source,
        account: accountMap.value.get(source.account_id) ?? null,
        rules: matchedRules,
        warnings: warningsFor(source, matchedRules),
      }
    }),
  )

  const orphanRules = computed(() => rules.value.filter((rule) => rule.source_ids.length === 0))

  const stats = computed(() => {
    const linkedSources = sourceNodes.value.filter((node) => node.rules.length > 0).length
    const enabledRules = rules.value.filter((rule) => rule.enabled).length
    const enabledSinks = sinks.value.filter((sink) => sink.enabled).length
    const warningSources = sourceNodes.value.filter((node) => node.warnings.length > 0).length
    return {
      linkedSources,
      warningSources,
      enabledRules,
      enabledSinks,
      totalSources: sources.value.length,
      totalRules: rules.value.length,
      totalSinks: sinks.value.length,
      totalTemplates: templates.value.length,
    }
  })

  async function load() {
    loading.value = true
    try {
      ;[accounts.value, sources.value, rules.value, sinks.value, templates.value] = await Promise.all([
        accountsApi.list(),
        sourcesApi.list(),
        rulesApi.list(),
        sinksApi.list(),
        templatesApi.list(),
      ])
    } finally {
      loading.value = false
    }
  }

  function warningsFor(source: Source, matchedRules: FlowRuleNode[]): string[] {
    const warnings: string[] = []
    if (!source.enabled) warnings.push('监听源已停用')
    if (matchedRules.length === 0) warnings.push('尚未关联规则')
    for (const item of matchedRules) {
      if (!item.rule.enabled) warnings.push(`规则「${item.rule.name}」已停用`)
      if (item.targets.length === 0) warnings.push(`规则「${item.rule.name}」没有目标渠道`)
      for (const target of item.targets) {
        if (!target.sink) warnings.push(`规则「${item.rule.name}」引用了不存在的渠道 #${target.sinkId}`)
        else if (!target.sink.enabled) warnings.push(`渠道「${target.sink.name}」已停用`)
        if (target.templateId && !target.template) {
          warnings.push(`规则「${item.rule.name}」引用了不存在的模板 #${target.templateId}`)
        }
      }
    }
    return [...new Set(warnings)]
  }

  return {
    accounts,
    sources,
    rules,
    sinks,
    templates,
    loading,
    sourceNodes,
    orphanRules,
    stats,
    load,
  }
}
