// Minimal toast queue — no dependencies. Components read `toasts` and call the
// helpers; <Toaster> renders them.
import { ref } from 'vue'

export type ToastKind = 'success' | 'error' | 'info'

export interface Toast {
  id: number
  kind: ToastKind
  message: string
}

export const toasts = ref<Toast[]>([])

let seq = 0

export function dismissToast(id: number) {
  toasts.value = toasts.value.filter((t) => t.id !== id)
}

export function pushToast(kind: ToastKind, message: string, ttl = 4000) {
  const id = ++seq
  toasts.value = [...toasts.value, { id, kind, message }]
  if (ttl > 0) window.setTimeout(() => dismissToast(id), ttl)
  return id
}

export const toast = {
  success: (m: string) => pushToast('success', m),
  error: (m: string) => pushToast('error', m, 6000),
  info: (m: string) => pushToast('info', m),
}
