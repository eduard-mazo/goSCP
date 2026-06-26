// Theme state shared across the app. The initial class is set in index.html
// before paint (no flash); this module keeps it reactive and persisted.
import { ref, watch } from 'vue'

export type Theme = 'light' | 'dark'

const KEY = 'goscp.theme'

function initial(): Theme {
  if (typeof document !== 'undefined' && document.documentElement.classList.contains('dark')) {
    return 'dark'
  }
  return 'light'
}

const theme = ref<Theme>(initial())

watch(
  theme,
  (t) => {
    document.documentElement.classList.toggle('dark', t === 'dark')
    try {
      localStorage.setItem(KEY, t)
    } catch {
      /* storage may be unavailable (private mode) — ignore */
    }
  },
  { immediate: true },
)

export function useTheme() {
  return {
    theme,
    toggle() {
      theme.value = theme.value === 'dark' ? 'light' : 'dark'
    },
  }
}
