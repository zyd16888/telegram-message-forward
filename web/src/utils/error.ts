// errText 从 axios 错误中提取后端返回的错误信息。
export function errText(e: unknown): string {
  const err = e as { response?: { data?: { error?: string } }; message?: string }
  return err.response?.data?.error ?? err.message ?? '未知错误'
}
