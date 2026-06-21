<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  ArrowLeft, Download, Trash2, FolderPlus, Upload, RefreshCw,
  Folder, File as FileIcon, Pencil, LogOut, HardDrive, Loader2, ChevronRight,
} from 'lucide-vue-next'
import { api, clearToken, type Entry, type Listing, type Usage } from '@/api/client'
import { cn, formatBytes, formatDate } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'

const emit = defineEmits<{ (e: 'logout'): void }>()

const listing = ref<Listing | null>(null)
const usage = ref<Usage | null>(null)
const loading = ref(false)
const error = ref('')
const dragging = ref(false)
const uploading = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

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

async function load(path = cwd.value) {
  loading.value = true
  error.value = ''
  try {
    listing.value = await api.list(path)
    usage.value = await api.usage()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load directory'
  } finally {
    loading.value = false
  }
}

function open(entry: Entry) {
  if (entry.isDir) load(entry.path)
  else download(entry)
}

async function download(entry: Entry) {
  try {
    await api.download(entry.path, entry.name)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Download failed'
  }
}

async function remove(entry: Entry) {
  if (!confirm(`Delete "${entry.name}"? This cannot be undone.`)) return
  try {
    await api.remove(entry.path)
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Delete failed'
  }
}

async function rename(entry: Entry) {
  const name = prompt('New name:', entry.name)
  if (!name || name === entry.name) return
  try {
    await api.rename(entry.path, name)
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Rename failed'
  }
}

async function makeDir() {
  const name = prompt('New folder name:')
  if (!name) return
  try {
    await api.mkdir(cwd.value, name)
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Create folder failed'
  }
}

async function doUpload(files: FileList | File[]) {
  if (!files || (files as FileList).length === 0) return
  uploading.value = true
  error.value = ''
  try {
    await api.upload(cwd.value, files)
    await load()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Upload failed'
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

onMounted(() => load('/'))
</script>

<template>
  <div
    class="min-h-screen bg-background"
    @dragover.prevent="dragging = true"
    @dragleave.prevent="dragging = false"
    @drop.prevent="onDrop"
  >
    <!-- Header -->
    <header class="sticky top-0 z-10 border-b bg-background/95 backdrop-blur">
      <div class="container flex h-14 items-center justify-between gap-4">
        <div class="flex items-center gap-2 font-semibold">
          <HardDrive class="h-5 w-5 text-primary" />
          <span>goSCP</span>
        </div>
        <div class="flex items-center gap-2 text-sm text-muted-foreground">
          <span v-if="usage" class="hidden sm:inline">
            {{ usage.fileCount }} files · {{ formatBytes(usage.totalSize) }}
          </span>
          <Button variant="ghost" size="sm" @click="logout">
            <LogOut /> <span class="hidden sm:inline">Logout</span>
          </Button>
        </div>
      </div>
    </header>

    <main class="container py-6">
      <!-- Toolbar -->
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <nav class="flex items-center gap-1 text-sm">
          <Button
            variant="ghost" size="icon"
            :disabled="cwd === '/'"
            @click="load(listing?.parent ?? '/')"
          >
            <ArrowLeft />
          </Button>
          <template v-for="(c, i) in crumbs" :key="c.path">
            <ChevronRight v-if="i > 0" class="h-3 w-3 text-muted-foreground" />
            <button
              class="rounded px-1.5 py-0.5 hover:bg-accent"
              :class="i === crumbs.length - 1 ? 'font-medium text-foreground' : 'text-muted-foreground'"
              @click="load(c.path)"
            >
              {{ c.name }}
            </button>
          </template>
        </nav>

        <div class="flex items-center gap-2">
          <Button variant="outline" size="sm" @click="load()">
            <RefreshCw :class="cn(loading && 'animate-spin')" /> Refresh
          </Button>
          <Button variant="outline" size="sm" @click="makeDir">
            <FolderPlus /> New folder
          </Button>
          <Button size="sm" :disabled="uploading" @click="fileInput?.click()">
            <Loader2 v-if="uploading" class="animate-spin" />
            <Upload v-else /> Upload
          </Button>
          <input ref="fileInput" type="file" multiple class="hidden" @change="onFilePick" />
        </div>
      </div>

      <p v-if="error" class="mb-4 rounded-md border border-destructive/50 bg-destructive/10 px-4 py-2 text-sm text-destructive">
        {{ error }}
      </p>

      <!-- File table -->
      <Card>
        <CardContent class="p-0">
          <table class="w-full text-sm">
            <thead class="border-b text-left text-muted-foreground">
              <tr>
                <th class="px-4 py-2.5 font-medium">Name</th>
                <th class="px-4 py-2.5 font-medium hidden sm:table-cell">Size</th>
                <th class="px-4 py-2.5 font-medium hidden md:table-cell">Modified</th>
                <th class="px-4 py-2.5 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading && !listing">
                <td colspan="4" class="px-4 py-10 text-center text-muted-foreground">
                  <Loader2 class="mx-auto h-5 w-5 animate-spin" />
                </td>
              </tr>
              <tr v-else-if="listing && listing.entries.length === 0">
                <td colspan="4" class="px-4 py-12 text-center text-muted-foreground">
                  This folder is empty. Drag files here or use Upload.
                </td>
              </tr>
              <tr
                v-for="entry in listing?.entries"
                :key="entry.path"
                class="border-b last:border-0 hover:bg-accent/50"
              >
                <td class="px-4 py-2">
                  <button class="flex items-center gap-2 text-left" @click="open(entry)">
                    <Folder v-if="entry.isDir" class="h-4 w-4 shrink-0 text-primary" />
                    <FileIcon v-else class="h-4 w-4 shrink-0 text-muted-foreground" />
                    <span class="truncate">{{ entry.name }}</span>
                  </button>
                </td>
                <td class="px-4 py-2 text-muted-foreground hidden sm:table-cell">
                  {{ entry.isDir ? '—' : formatBytes(entry.size) }}
                </td>
                <td class="px-4 py-2 text-muted-foreground hidden md:table-cell">
                  {{ formatDate(entry.modTime) }}
                </td>
                <td class="px-4 py-2">
                  <div class="flex justify-end gap-1">
                    <Button v-if="!entry.isDir" variant="ghost" size="icon" title="Download" @click="download(entry)">
                      <Download />
                    </Button>
                    <Button variant="ghost" size="icon" title="Rename" @click="rename(entry)">
                      <Pencil />
                    </Button>
                    <Button variant="ghost" size="icon" title="Delete" @click="remove(entry)">
                      <Trash2 class="text-destructive" />
                    </Button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </CardContent>
      </Card>
    </main>

    <!-- Drag overlay -->
    <div
      v-if="dragging"
      class="pointer-events-none fixed inset-0 z-50 flex items-center justify-center bg-primary/10 backdrop-blur-sm"
    >
      <div class="rounded-xl border-2 border-dashed border-primary bg-background p-12 text-center">
        <Upload class="mx-auto mb-3 h-10 w-10 text-primary" />
        <p class="text-lg font-medium">Drop files to upload</p>
        <p class="text-sm text-muted-foreground">to {{ cwd }}</p>
      </div>
    </div>
  </div>
</template>
