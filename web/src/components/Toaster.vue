<script setup lang="ts">
import { CircleCheck, CircleAlert, Info, X } from 'lucide-vue-next'
import { toasts, dismissToast, type ToastKind } from '@/lib/toast'

const meta: Record<ToastKind, { icon: typeof Info; cls: string }> = {
  success: { icon: CircleCheck, cls: 'text-[hsl(var(--success))]' },
  error: { icon: CircleAlert, cls: 'text-destructive' },
  info: { icon: Info, cls: 'text-primary' },
}
</script>

<template>
  <Teleport to="body">
    <div class="pointer-events-none fixed inset-x-0 bottom-0 z-[60] flex flex-col items-center gap-2 p-4 sm:inset-x-auto sm:right-0 sm:top-0 sm:bottom-auto sm:items-end">
      <TransitionGroup
        enter-active-class="animate-toast-in"
        leave-active-class="transition-all duration-200"
        leave-to-class="opacity-0 translate-x-4"
      >
        <div
          v-for="t in toasts"
          :key="t.id"
          class="pointer-events-auto flex w-full max-w-sm items-start gap-3 rounded-lg border bg-popover/95 px-4 py-3 text-sm text-popover-foreground shadow-lg backdrop-blur"
        >
          <component
            :is="meta[t.kind].icon"
            class="mt-0.5 h-4 w-4 shrink-0"
            :class="meta[t.kind].cls"
          />
          <p class="flex-1 leading-snug">
            {{ t.message }}
          </p>
          <button
            class="shrink-0 rounded text-muted-foreground transition-colors hover:text-foreground"
            aria-label="Dismiss"
            @click="dismissToast(t.id)"
          >
            <X class="h-4 w-4" />
          </button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>
