<script setup lang="ts">
import { Sun, Moon } from 'lucide-vue-next'
import { useTheme } from '@/lib/theme'

const { theme, toggle } = useTheme()
</script>

<template>
  <button
    type="button"
    class="relative inline-flex h-9 w-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    :aria-label="theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'"
    :title="theme === 'dark' ? 'Light mode' : 'Dark mode'"
    @click="toggle"
  >
    <Transition
      mode="out-in"
      enter-active-class="transition duration-200"
      enter-from-class="opacity-0 -rotate-90 scale-50"
      leave-active-class="transition duration-200"
      leave-to-class="opacity-0 rotate-90 scale-50"
    >
      <Sun
        v-if="theme === 'dark'"
        key="sun"
        class="h-[1.15rem] w-[1.15rem]"
      />
      <Moon
        v-else
        key="moon"
        class="h-[1.15rem] w-[1.15rem]"
      />
    </Transition>
  </button>
</template>
