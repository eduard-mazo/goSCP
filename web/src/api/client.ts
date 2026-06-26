// Typed client for the goSCP backend API. The bearer token is held in
// localStorage so it survives reloads but never leaves the browser.

export interface Entry {
  name: string
  path: string
  isDir: boolean
  size: number
  modTime: string
  mode: string
}

export interface Listing {
  path: string
  parent: string
  entries: Entry[]
}

export interface Usage {
  root: string
  totalSize: number
  fileCount: number
  dirCount: number
}

/** Progress callback for uploads: 0..1, plus byte counts. */
export type ProgressFn = (ratio: number, loaded: number, total: number) => void

const TOKEN_KEY = 'goscp.token'

export function getToken(): string {
  return localStorage.getItem(TOKEN_KEY) ?? ''
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers)
  headers.set('Authorization', `Bearer ${getToken()}`)

  const res = await fetch(`/api/v1${path}`, { ...init, headers })
  if (res.status === 204) return undefined as T
  const body = await res.json().catch(() => ({}))
  if (!res.ok) {
    throw new ApiError(res.status, body?.error ?? res.statusText)
  }
  return body as T
}

export const api = {
  /** Validate the current token by hitting an authenticated endpoint. */
  async checkAuth(): Promise<boolean> {
    try {
      await request('/usage')
      return true
    } catch (e) {
      if (e instanceof ApiError && e.status === 401) return false
      throw e
    }
  },

  usage: () => request<Usage>('/usage'),

  /** Recursively computed size + counts for a single directory (or file). */
  dirSize: (path: string) =>
    request<Usage>(`/dirsize?path=${encodeURIComponent(path)}`),

  list: (path: string) =>
    request<Listing>(`/files?path=${encodeURIComponent(path)}`),

  mkdir: (path: string, name: string) =>
    request<Entry>('/mkdir', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path, name }),
    }),

  rename: (path: string, name: string) =>
    request<Entry>('/rename', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path, name }),
    }),

  remove: (path: string) =>
    request<void>(`/files?path=${encodeURIComponent(path)}`, {
      method: 'DELETE',
    }),

  /** Build a download URL with the token embedded as a header via fetch. */
  async download(path: string, name: string): Promise<void> {
    const res = await fetch(`/api/v1/download?path=${encodeURIComponent(path)}`, {
      headers: { Authorization: `Bearer ${getToken()}` },
    })
    if (!res.ok) throw new ApiError(res.status, 'download failed')
    const blob = await res.blob()
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = name
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  },

  /**
   * Upload one or more files into the given directory. Uses XHR (not fetch) so
   * we can surface real upload progress to the UI.
   */
  upload(dir: string, files: FileList | File[], onProgress?: ProgressFn): Promise<void> {
    return new Promise((resolve, reject) => {
      const form = new FormData()
      form.append('path', dir)
      for (const f of Array.from(files)) form.append('files', f)

      const xhr = new XMLHttpRequest()
      xhr.open('POST', '/api/v1/upload')
      xhr.setRequestHeader('Authorization', `Bearer ${getToken()}`)

      xhr.upload.onprogress = (e) => {
        if (onProgress && e.lengthComputable) {
          onProgress(e.loaded / e.total, e.loaded, e.total)
        }
      }
      xhr.onerror = () => reject(new ApiError(0, 'network error during upload'))
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve()
          return
        }
        let msg = 'upload failed'
        try {
          msg = JSON.parse(xhr.responseText)?.error ?? msg
        } catch {
          /* non-JSON error body */
        }
        reject(new ApiError(xhr.status, msg))
      }
      xhr.send(form)
    })
  },
}
