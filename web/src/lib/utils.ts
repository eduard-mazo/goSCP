import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** cn merges conditional class names and resolves Tailwind conflicts. */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/** Human-readable byte size with one decimal of precision. */
export function formatBytes(bytes: number): string {
  if (!bytes || bytes < 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.min(sizes.length - 1, Math.floor(Math.log(bytes) / Math.log(k)))
  const n = bytes / Math.pow(k, i)
  return `${i === 0 ? n : parseFloat(n.toFixed(1))} ${sizes[i]}`
}

/** Short, locale-aware absolute date/time, e.g. "Jun 22, 2026, 14:05". */
export function formatDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  return d.toLocaleString(undefined, {
    year: 'numeric', month: 'short', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

/** Compact relative time, e.g. "3 min ago", "yesterday", "2 mo ago". */
export function formatRelative(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  const diff = Date.now() - d.getTime()
  const sec = Math.round(diff / 1000)
  if (sec < 45) return 'just now'
  const min = Math.round(sec / 60)
  if (min < 60) return `${min} min ago`
  const hr = Math.round(min / 60)
  if (hr < 24) return `${hr} hr ago`
  const day = Math.round(hr / 24)
  if (day === 1) return 'yesterday'
  if (day < 30) return `${day} days ago`
  const mo = Math.round(day / 30)
  if (mo < 12) return `${mo} mo ago`
  return `${Math.round(mo / 12)} yr ago`
}

/**
 * Convert a Go os.FileMode string (e.g. "-rw-r--r--" or "drwxr-xr-x") into a
 * numeric octal permission string (e.g. "644", "755"). Returns "" if it can't
 * be parsed from the trailing rwx triplets.
 */
export function modeToOctal(mode: string): string {
  const m = mode.match(/[rwxst-]{9}$/)
  if (!m) return ''
  const perm = m[0]
  let out = ''
  for (let i = 0; i < 9; i += 3) {
    const t = perm.slice(i, i + 3)
    out += String(
      (t[0] !== '-' ? 4 : 0) + (t[1] !== '-' ? 2 : 0) + (t[2] !== '-' ? 1 : 0),
    )
  }
  return out
}

/** Strip the leading type byte from a mode string, leaving only "rwxr-xr-x". */
export function permString(mode: string): string {
  const m = mode.match(/[rwxst-]{9}$/)
  return m ? m[0] : mode
}
