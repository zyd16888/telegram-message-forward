export interface DateTimeFormatOptions {
  timeZone?: string
  seconds?: boolean
  dateStyle?: 'full' | 'long' | 'medium' | 'short'
}

function validDate(value?: string | null): Date | null {
  if (!value) return null
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}

export function resolvedTimeZone(timeZone?: string): string {
  if (timeZone) {
    try {
      new Intl.DateTimeFormat('zh-CN', { timeZone }).format()
      return timeZone
    } catch {
      // Fall through to the browser time zone for stale or invalid profile data.
    }
  }
  return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
}

export function formatDateTime(value?: string | null, options: DateTimeFormatOptions = {}): string {
  const date = validDate(value)
  if (!date) return '-'
  return new Intl.DateTimeFormat('zh-CN', {
    timeZone: resolvedTimeZone(options.timeZone),
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: options.seconds === false ? undefined : '2-digit',
    hourCycle: 'h23',
  }).format(date)
}

export function formatDateTimeTitle(value?: string | null, timeZone?: string): string {
  const formatted = formatDateTime(value, { timeZone })
  if (formatted === '-') return formatted
  return `${formatted} (${resolvedTimeZone(timeZone)})`
}

export function formatDuration(start?: string | null, end?: string | null): string {
  const startDate = validDate(start)
  const endDate = validDate(end)
  if (!startDate || !endDate) return '-'
  const milliseconds = Math.max(0, endDate.getTime() - startDate.getTime())
  if (milliseconds < 1000) return `${milliseconds} ms`
  const seconds = Math.round(milliseconds / 1000)
  if (seconds < 60) return `${seconds} 秒`
  const minutes = Math.floor(seconds / 60)
  const remainSeconds = seconds % 60
  return remainSeconds ? `${minutes} 分 ${remainSeconds} 秒` : `${minutes} 分钟`
}

export function runDisplayTime(run: { finished_at?: string; started_at?: string; created_at: string }): string {
  return run.finished_at || run.started_at || run.created_at
}
