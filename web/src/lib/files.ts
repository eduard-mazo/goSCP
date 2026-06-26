// File-type detection → icon component + tint class. The small palette of
// per-category tints makes folders and file kinds scannable at a glance while
// staying within the warm, amber-anchored theme.
import type { Component } from 'vue'
import {
  File as FileIcon, FileText, FileCode, FileImage,
  FileArchive, FileAudio, FileVideo, FileJson,
} from 'lucide-vue-next'

export interface FileKind {
  icon: Component
  /** Tailwind text-color class for the icon tint. */
  tint: string
  label: string
}

const IMAGE = /\.(png|jpe?g|gif|webp|svg|avif|bmp|ico|heic|tiff?)$/i
const VIDEO = /\.(mp4|mkv|mov|avi|webm|m4v|flv|wmv)$/i
const AUDIO = /\.(mp3|wav|flac|aac|ogg|m4a|opus|wma)$/i
const ARCHIVE = /\.(zip|tar|gz|tgz|bz2|xz|7z|rar|zst)$/i
const CODE = /\.(go|js|ts|tsx|jsx|vue|py|rs|c|h|cpp|cc|java|rb|php|sh|bash|zsh|sql|html?|css|scss|yml|yaml|toml|ini|conf|lua|swift|kt)$/i
const DOC = /\.(txt|md|markdown|rst|log|pdf|docx?|rtf|csv|xlsx?|pptx?)$/i
const JSONX = /\.(json|jsonc|geojson)$/i

export function fileKind(name: string): FileKind {
  if (JSONX.test(name)) return { icon: FileJson, tint: 'text-amber-500', label: 'JSON' }
  if (IMAGE.test(name)) return { icon: FileImage, tint: 'text-violet-500', label: 'Image' }
  if (VIDEO.test(name)) return { icon: FileVideo, tint: 'text-rose-500', label: 'Video' }
  if (AUDIO.test(name)) return { icon: FileAudio, tint: 'text-pink-500', label: 'Audio' }
  if (ARCHIVE.test(name)) return { icon: FileArchive, tint: 'text-orange-500', label: 'Archive' }
  if (CODE.test(name)) return { icon: FileCode, tint: 'text-sky-500', label: 'Code' }
  if (DOC.test(name)) return { icon: FileText, tint: 'text-emerald-500', label: 'Document' }
  return { icon: FileIcon, tint: 'text-muted-foreground', label: 'File' }
}
