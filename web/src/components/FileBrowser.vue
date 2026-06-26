<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  CornerLeftUp, Download, Trash2, FolderPlus, Upload, RefreshCw,
  Folder, Pencil, LogOut, HardDrive, Loader2, ChevronRight,
  Search, X, ArrowUp, ArrowDown, ArrowUpDown, Server, FolderTree,
} from 'lucide-vue-next'
import { api, clearToken, type Entry, type Listing, type Usage } from '@/api/client'
import { cn, formatBytes, formatDate, formatRelative, modeToOctal, permString } from '@/lib/utils'
import { fileKind } from '@/lib/files'
import { toast } from '@/lib/toast'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Progress } from '@/components/ui/progress'
import { Dialog } from '@/components/ui/dialog'
import ThemeToggle from '@/components/ThemeToggle.vue'

const emit = defineEmits<{ (e: 'logout'): void }>()

const listing = ref<Listing | null>(null)
const usage = ref<Usage | null>(null)
const loading = ref(false)
const error = ref('')
const dragging = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

// Upload progress
const uploading = ref(false)
const uploadPct = ref(0)
const uploadInfo = ref('')

// View controls
const filter = ref('')
type SortKey = 'name' | 'size' | 'modified'
const sortKey = ref<SortKey>('name')
const sortDir = ref<'asc' | 'desc'>('asc')

// Lazily computed folder sizes, keyed by path. Reset on navigation.
const dirSizes = ref(new Map<string, Usage>())
const dirSizeLoading = ref(new Set<string>())
let loadGen = 0

const cwd = computed(() => listing.value?.path ?? '/')

const crumbs = computed(() => {
  const parts = cwd.value.split('/').filter(Boolean)
  const acc: { name: string; path: string }[] = [{ name: 'root', path: '/' }]
  let cur = ''
  for (const p of parts) {
    cur += '/' + p
    acc.push({ name: p, path: cur })
  }
  return acc
})

const visibleEntries = computed(() => {
  const all = listing.value?.entries ?? []
  const q = filter.value.trim().toLowerCase()
  const filtered = q ? all.filter((e) => e.name.toLowerCase().includes(q)) : all
  const factor = sortDir.value === 'asc' ? 1 : -1
  const sizeOf = (e: Entry) => (e.isDir ? dirSizes.value.get(e.path)?.totalSize ?? -1 : e.size)
  return [...filtered].sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1 // folders first, always
    let cmp = 0
    if (sortKey.value === 'size') cmp = sizeOf(a) - sizeOf(b)
    else if (sortKey.value === 'modified') cmp = +new Date(a.modTime) - +new Date(b.modTime)
    if (cmp === 0) cmp = a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' })
    return cmp * (sortKey.value === 'name' ? (sortDir.value === 'asc' ? 1 : -1) : factor)
  })
})

function setSort(key: SortKey) {
  if (sortKey.value === key) sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  else {
    sortKey.value = key
    sortDir.value = key === 'name' ? 'asc' : 'desc'
  }
}

async function load(path = cwd.value) {
  loading.value = true
  error.value = ''
  const gen = ++loadGen
  dirSizes.value = new Map()
  dirSizeLoading.value = new Set()
  try {
    const [l, u] = await Promise.all([api.list(path), api.usage()])
    if (gen !== loadGen) return
    listing.value = l
    usage.value = u
    void loadDirSizes(l.entries, gen)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load directory'
  } finally {
    if (gen === loadGen) loading.value = false
  }
}

// Fetch folder sizes a few at a time so listings stay snappy and we never
// flood the backend with one request per subtree at once.
async function loadDirSizes(entries: Entry[], gen: number) {
  const dirs = entries.filter((e) => e.isDir)
  let i = 0
  const worker = async () => {
    while (i < dirs.length && gen === loadGen) {
      const e = dirs[i++]
      dirSizeLoading.value.add(e.path)
      try {
        const u = await api.dirSize(e.path)
        if (gen === loadGen) dirSizes.value.set(e.path, u)
      } catch {
        /* leave size unknown */
      } finally {
        dirSizeLoading.value.delete(e.path)
      }
    }
  }
  await Promise.all(Array.from({ length: 4 }, worker))
}

function open(entry: Entry) {
  if (entry.isDir) load(entry.path)
  else download(entry)
}

async function download(entry: Entry) {
  try {
    await api.download(entry.path, entry.name)
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Download failed')
  }
}

// ── Dialog-driven actions (replaces native prompt/confirm) ──────────────────
type DialogKind = 'mkdir' | 'rename' | 'delete'
const dialog = ref<{ kind: DialogKind; entry: Entry | null; value: string; busy: boolean } | null>(null)

function askMkdir() {
  dialog.value = { kind: 'mkdir', entry: null, value: '', busy: false }
}
function askRename(entry: Entry) {
  dialog.value = { kind: 'rename', entry, value: entry.name, busy: false }
}
function askDelete(entry: Entry) {
  dialog.value = { kind: 'delete', entry, value: '', busy: false }
}
function closeDialog() {
  if (!dialog.value?.busy) dialog.value = null
}

async function confirmDialog() {
  const d = dialog.value
  if (!d) return
  d.busy = true
  try {
    if (d.kind === 'mkdir') {
      const name = d.value.trim()
      if (!name) { d.busy = false; return }
      await api.mkdir(cwd.value, name)
      toast.success(`Folder “${name}” created`)
    } else if (d.kind === 'rename' && d.entry) {
      const name = d.value.trim()
      if (!name || name === d.entry.name) { dialog.value = null; return }
      await api.rename(d.entry.path, name)
      toast.success(`Renamed to “${name}”`)
    } else if (d.kind === 'delete' && d.entry) {
      await api.remove(d.entry.path)
      toast.success(`Deleted “${d.entry.name}”`)
    }
    dialog.value = null
    await load()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Operation failed')
    if (dialog.value) dialog.value.busy = false
  }
}

// ── Upload ──────────────────────────────────────────────────────────────────
async function doUpload(files: FileList | File[]) {
  const list = Array.from(files)
  if (list.length === 0) return
  uploading.value = true
  uploadPct.value = 0
  uploadInfo.value = `0 / ${list.length} files`
  try {
    await api.upload(cwd.value, list, (ratio, loaded, total) => {
      uploadPct.value = ratio
      uploadInfo.value = `${formatBytes(loaded)} / ${formatBytes(total)}`
    })
    toast.success(`Uploaded ${list.length} ${list.length === 1 ? 'file' : 'files'}`)
    await load()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : 'Upload failed')
  } finally {
    uploading.value = false
  }
}

function onFilePick(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files) doUpload(input.files)
  input.value = ''
}

function onDrop(e: DragEvent) {
  dragging.value = false
  if (e.dataTransfer?.files) doUpload(e.dataTransfer.files)
}

function logout() {
  clearToken()
  emit('logout')
}

// Display helpers
function sizeLabel(entry: Entry): string {
  if (!entry.isDir) return formatBytes(entry.size)
  const u = dirSizes.value.get(entry.path)
  return u ? formatBytes(u.totalSize) : ''
}
function dirCountLabel(entry: Entry): string {
  const u = dirSizes.value.get(entry.path)
  if (!u) return ''
  const f = `${u.fileCount} ${u.fileCount === 1 ? 'file' : 'files'}`
  return u.dirCount ? `${f}, ${u.dirCount} ${u.dirCount === 1 ? 'folder' : 'folders'}` : f
}

onMounted(() => load('/'))
</script>

<template>
  <div
    class="flex min-h-[100dvh] flex-col"
    @dragover.prevent="dragging = true"
    @dragleave.prevent="dragging = false"
    @drop.prevent="onDrop"
  >
    <!-- Header -->
    <header class="sticky top-0 z-20 border-b border-border/70 bg-background/80 backdrop-blur-md">
      <div class="mx-auto flex h-16 w-full max-w-6xl items-center justify-between gap-4 px-4 sm:px-6">
        <div class="flex min-w-0 items-center gap-3">
          <div class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border border-primary/30 bg-primary/10">
            <HardDrive class="h-5 w-5 text-primary" />
          </div>
          <div class="min-w-0">
            <p class="font-mono text-base font-semibold leading-none tracking-tight">
              goSCP
            </p>
            <p
              v-if="usage"
              class="mt-1 hidden truncate font-mono text-xs text-muted-foreground sm:block"
              :title="usage.root"
            >
              {{ usage.root }}
            </p>
          </div>
        </div>

        <div class="flex items-center gap-1 sm:gap-2">
          <div
            v-if="usage"
            class="mr-1 hidden items-center gap-3 rounded-lg border bg-card/60 px-3 py-1.5 font-mono text-xs text-muted-foreground md:flex"
          >
            <span class="flex items-center gap-1.5"><Server class="h-3.5 w-3.5" />{{ usage.fileCount }}</span>
            <span class="text-border">·</span>
            <span class="flex items-center gap-1.5"><Folder class="h-3.5 w-3.5" />{{ usage.dirCount }}</span>
            <span class="text-border">·</span>
            <span class="font-semibold text-foreground">{{ formatBytes(usage.totalSize) }}</span>
          </div>
          <ThemeToggle />
          <Button
            variant="ghost"
            size="sm"
            @click="logout"
          >
            <LogOut /> <span class="hidden sm:inline">Logout</span>
          </Button>
        </div>
      </div>
    </header>

    <main class="mx-auto w-full max-w-6xl flex-1 px-4 py-5 sm:px-6 sm:py-7">
      <!-- Toolbar -->
      <div class="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <!-- Breadcrumb -->
        <nav class="flex min-w-0 items-center gap-1 text-sm">
          <Button
            variant="ghost"
            size="icon"
            class="shrink-0"
            :disabled="cwd === '/'"
            title="Up one level"
            @click="load(listing?.parent ?? '/')"
          >
            <CornerLeftUp />
          </Button>
          <div class="flex min-w-0 items-center gap-0.5 overflow-x-auto rounded-md border bg-card/60 px-2 py-1.5">
            <template
              v-for="(c, i) in crumbs"
              :key="c.path"
            >
              <ChevronRight
                v-if="i > 0"
                class="h-3.5 w-3.5 shrink-0 text-muted-foreground/60"
              />
              <button
                class="shrink-0 rounded px-1.5 py-0.5 font-mono text-xs transition-colors hover:bg-accent"
                :class="i === crumbs.length - 1 ? 'font-semibold text-foreground' : 'text-muted-foreground'"
                @click="load(c.path)"
              >
                {{ c.name }}
              </button>
            </template>
          </div>
        </nav>

        <!-- Actions -->
        <div class="flex flex-wrap items-center gap-2">
          <div class="relative min-w-0 flex-1 sm:flex-none">
            <Search class="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              v-model="filter"
              placeholder="Filter…"
              class="h-9 w-full pl-8 pr-8 sm:w-44"
            />
            <button
              v-if="filter"
              class="absolute right-2 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              aria-label="Clear filter"
              @click="filter = ''"
            >
              <X class="h-4 w-4" />
            </button>
          </div>
          <Button
            variant="outline"
            size="sm"
            :disabled="loading"
            title="Refresh"
            @click="load()"
          >
            <RefreshCw :class="cn(loading && 'animate-spin')" /> <span class="hidden sm:inline">Refresh</span>
          </Button>
          <Button
            variant="outline"
            size="sm"
            @click="askMkdir"
          >
            <FolderPlus /> <span class="hidden sm:inline">New folder</span>
          </Button>
          <Button
            size="sm"
            :disabled="uploading"
            @click="fileInput?.click()"
          >
            <Loader2
              v-if="uploading"
              class="animate-spin"
            />
            <Upload v-else /> Upload
          </Button>
          <input
            ref="fileInput"
            type="file"
            multiple
            class="hidden"
            @change="onFilePick"
          >
        </div>
      </div>

      <!-- Upload progress -->
      <Transition
        enter-active-class="transition-all duration-200"
        enter-from-class="opacity-0 -translate-y-1"
        leave-active-class="transition-all duration-200"
        leave-to-class="opacity-0"
      >
        <div
          v-if="uploading"
          class="mb-4 rounded-lg border bg-card p-3 shadow-sm"
        >
          <div class="mb-2 flex items-center justify-between text-sm">
            <span class="flex items-center gap-2 font-medium"><Upload class="h-4 w-4 text-primary" /> Uploading…</span>
            <span class="font-mono text-xs text-muted-foreground tabular">{{ uploadInfo }} · {{ Math.round(uploadPct * 100) }}%</span>
          </div>
          <Progress :value="uploadPct" />
        </div>
      </Transition>

      <p
        v-if="error"
        class="mb-4 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-2 text-sm text-destructive"
      >
        {{ error }}
      </p>

      <!-- Listing -->
      <div class="overflow-hidden rounded-xl border bg-card shadow-sm">
        <!-- Column header (desktop) -->
        <div class="hidden items-center gap-3 border-b bg-muted/30 px-4 py-2.5 md:flex">
          <button
            class="flex flex-1 items-center gap-1.5 label-mono text-muted-foreground transition-colors hover:text-foreground"
            @click="setSort('name')"
          >
            Name
            <component
              :is="sortKey === 'name' ? (sortDir === 'asc' ? ArrowUp : ArrowDown) : ArrowUpDown"
              class="h-3 w-3"
              :class="sortKey !== 'name' && 'opacity-40'"
            />
          </button>
          <button
            class="flex w-24 items-center justify-end gap-1.5 label-mono text-muted-foreground transition-colors hover:text-foreground"
            @click="setSort('size')"
          >
            Size
            <component
              :is="sortKey === 'size' ? (sortDir === 'asc' ? ArrowUp : ArrowDown) : ArrowUpDown"
              class="h-3 w-3"
              :class="sortKey !== 'size' && 'opacity-40'"
            />
          </button>
          <div class="hidden w-40 label-mono text-muted-foreground lg:block">
            Permissions
          </div>
          <button
            class="flex w-36 items-center gap-1.5 label-mono text-muted-foreground transition-colors hover:text-foreground"
            @click="setSort('modified')"
          >
            Modified
            <component
              :is="sortKey === 'modified' ? (sortDir === 'asc' ? ArrowUp : ArrowDown) : ArrowUpDown"
              class="h-3 w-3"
              :class="sortKey !== 'modified' && 'opacity-40'"
            />
          </button>
          <div class="w-[104px] text-right label-mono text-muted-foreground">
            Actions
          </div>
        </div>

        <!-- Loading -->
        <div
          v-if="loading && !listing"
          class="flex items-center justify-center px-4 py-16 text-muted-foreground"
        >
          <Loader2 class="h-5 w-5 animate-spin" />
        </div>

        <!-- Empty -->
        <div
          v-else-if="visibleEntries.length === 0"
          class="flex flex-col items-center justify-center gap-3 px-4 py-16 text-center"
        >
          <div class="flex h-12 w-12 items-center justify-center rounded-full bg-muted">
            <FolderTree class="h-6 w-6 text-muted-foreground" />
          </div>
          <p class="text-sm text-muted-foreground">
            {{ filter ? `No entries match “${filter}”.` : 'This folder is empty. Drag files here or use Upload.' }}
          </p>
        </div>

        <!-- Rows -->
        <ul v-else>
          <li
            v-for="(entry, idx) in visibleEntries"
            :key="entry.path"
            class="row-in group flex items-center gap-3 border-b border-border/60 px-3 py-2.5 transition-colors last:border-0 hover:bg-accent/40 sm:px-4"
            :style="{ animationDelay: Math.min(idx, 16) * 22 + 'ms' }"
          >
            <!-- Name + mobile meta -->
            <button
              class="flex min-w-0 flex-1 items-center gap-3 text-left"
              @click="open(entry)"
            >
              <span
                class="flex h-9 w-9 shrink-0 items-center justify-center rounded-md"
                :class="entry.isDir ? 'bg-primary/10' : 'bg-muted'"
              >
                <Folder
                  v-if="entry.isDir"
                  class="h-[1.15rem] w-[1.15rem] text-primary"
                />
                <component
                  :is="fileKind(entry.name).icon"
                  v-else
                  class="h-[1.15rem] w-[1.15rem]"
                  :class="fileKind(entry.name).tint"
                />
              </span>
              <span class="min-w-0">
                <span class="block truncate text-sm font-medium group-hover:text-foreground">{{ entry.name }}</span>
                <!-- compact meta for small screens -->
                <span class="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 font-mono text-[0.7rem] text-muted-foreground md:hidden">
                  <span class="tabular">{{ sizeLabel(entry) || (entry.isDir ? '…' : '0 B') }}</span>
                  <span class="text-border">·</span>
                  <span>{{ permString(entry.mode) }}</span>
                  <span class="text-border">·</span>
                  <span>{{ formatRelative(entry.modTime) }}</span>
                </span>
              </span>
            </button>

            <!-- Size (desktop) -->
            <div class="hidden w-24 justify-end text-right font-mono text-xs text-muted-foreground md:flex">
              <span
                v-if="!entry.isDir"
                class="tabular"
              >{{ formatBytes(entry.size) }}</span>
              <span
                v-else-if="dirSizes.get(entry.path)"
                class="tabular"
                :title="dirCountLabel(entry)"
              >{{ formatBytes(dirSizes.get(entry.path)!.totalSize) }}</span>
              <Loader2
                v-else-if="dirSizeLoading.has(entry.path)"
                class="h-3.5 w-3.5 animate-spin opacity-60"
              />
              <span
                v-else
                class="opacity-40"
              >—</span>
            </div>

            <!-- Permissions (desktop, lg+) -->
            <div class="hidden w-40 items-center gap-1.5 lg:flex">
              <Badge variant="mono">
                {{ permString(entry.mode) }}
              </Badge>
              <span
                v-if="modeToOctal(entry.mode)"
                class="font-mono text-[0.7rem] text-muted-foreground/70"
              >{{ modeToOctal(entry.mode) }}</span>
            </div>

            <!-- Modified (desktop) -->
            <div
              class="hidden w-36 font-mono text-xs text-muted-foreground md:block"
              :title="formatDate(entry.modTime)"
            >
              {{ formatRelative(entry.modTime) }}
            </div>

            <!-- Actions -->
            <div class="flex shrink-0 items-center gap-0.5 opacity-70 transition-opacity group-hover:opacity-100 sm:w-[104px] sm:justify-end">
              <Button
                v-if="!entry.isDir"
                variant="ghost"
                size="icon"
                class="h-8 w-8"
                title="Download"
                @click.stop="download(entry)"
              >
                <Download />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                class="h-8 w-8"
                title="Rename"
                @click.stop="askRename(entry)"
              >
                <Pencil />
              </Button>
              <Button
                variant="ghost"
                size="icon"
                class="h-8 w-8 text-muted-foreground hover:text-destructive"
                title="Delete"
                @click.stop="askDelete(entry)"
              >
                <Trash2 />
              </Button>
            </div>
          </li>
        </ul>
      </div>

      <p class="mt-3 px-1 font-mono text-xs text-muted-foreground">
        {{ visibleEntries.length }}{{ filter ? ` of ${listing?.entries.length ?? 0}` : '' }}
        {{ (listing?.entries.length ?? 0) === 1 && !filter ? 'entry' : 'entries' }}
      </p>
    </main>

    <!-- Drag overlay -->
    <Transition
      enter-active-class="transition-opacity duration-150"
      enter-from-class="opacity-0"
      leave-active-class="transition-opacity duration-150"
      leave-to-class="opacity-0"
    >
      <div
        v-if="dragging"
        class="pointer-events-none fixed inset-0 z-50 flex items-center justify-center bg-background/70 p-6 backdrop-blur-sm"
      >
        <div class="flex flex-col items-center rounded-2xl border-2 border-dashed border-primary bg-card/90 px-12 py-10 text-center shadow-2xl">
          <div class="mb-3 flex h-14 w-14 items-center justify-center rounded-full bg-primary/15">
            <Upload class="h-7 w-7 text-primary" />
          </div>
          <p class="text-lg font-semibold">
            Drop to upload
          </p>
          <p class="mt-1 font-mono text-sm text-muted-foreground">
            → {{ cwd }}
          </p>
        </div>
      </div>
    </Transition>

    <!-- Action dialog -->
    <Dialog
      :open="!!dialog"
      @update:open="(v) => !v && closeDialog()"
    >
      <div
        v-if="dialog"
        class="p-6"
      >
        <h2 class="text-lg font-semibold">
          {{ dialog.kind === 'mkdir' ? 'New folder' : dialog.kind === 'rename' ? 'Rename' : 'Delete entry' }}
        </h2>

        <template v-if="dialog.kind === 'delete' && dialog.entry">
          <p class="mt-2 text-sm text-muted-foreground">
            Permanently delete
            <span class="font-medium text-foreground">“{{ dialog.entry.name }}”</span>?
            <template v-if="dialog.entry.isDir">
              All contents will be removed.
            </template>
            This cannot be undone.
          </p>
        </template>
        <template v-else>
          <p class="mt-2 text-sm text-muted-foreground">
            {{ dialog.kind === 'mkdir' ? 'Create a new folder in the current directory.' : 'Enter a new name for this entry.' }}
          </p>
          <Input
            v-model="dialog.value"
            class="mt-4 h-10 font-mono"
            :placeholder="dialog.kind === 'mkdir' ? 'folder-name' : 'new-name'"
            @keyup.enter="confirmDialog"
          />
        </template>

        <div class="mt-6 flex justify-end gap-2">
          <Button
            variant="ghost"
            :disabled="dialog.busy"
            @click="closeDialog"
          >
            Cancel
          </Button>
          <Button
            :variant="dialog.kind === 'delete' ? 'destructive' : 'default'"
            :disabled="dialog.busy || (dialog.kind !== 'delete' && !dialog.value.trim())"
            @click="confirmDialog"
          >
            <Loader2
              v-if="dialog.busy"
              class="animate-spin"
            />
            {{ dialog.kind === 'mkdir' ? 'Create' : dialog.kind === 'rename' ? 'Rename' : 'Delete' }}
          </Button>
        </div>
      </div>
    </Dialog>
  </div>
</template>
