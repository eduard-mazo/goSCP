<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{ open: boolean; closeOnBackdrop?: boolean }>(),
  { closeOnBackdrop: true },
)
const emit = defineEmits<{ (e: 'update:open', value: boolean): void }>()

const panel = ref<HTMLElement | null>(null)

function close() {
  emit('update:open', false)
}

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape') close()
}

watch(
  () => props.open,
  async (open) => {
    if (open) {
      document.addEventListener('keydown', onKey)
      document.body.style.overflow = 'hidden'
      await nextTick()
      // Focus the first input if present, otherwise the panel itself.
      const el = panel.value?.querySelector<HTMLElement>(
        'input, textarea, [autofocus]',
      )
      ;(el ?? panel.value)?.focus()
    } else {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = ''
    }
  },
)

onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKey)
  document.body.style.overflow = ''
})
</script>

<template>
  <Teleport to="body">
    <Transition
      enter-active-class="transition-opacity duration-150"
      enter-from-class="opacity-0"
      leave-active-class="transition-opacity duration-150"
      leave-to-class="opacity-0"
    >
      <div
        v-if="open"
        class="fixed inset-0 z-50 flex items-end justify-center bg-black/60 p-4 backdrop-blur-sm sm:items-center"
        @click.self="closeOnBackdrop && close()"
      >
        <div
          ref="panel"
          tabindex="-1"
          role="dialog"
          aria-modal="true"
          class="w-full max-w-md origin-bottom rounded-xl border bg-card text-card-foreground shadow-2xl outline-none animate-scale-in sm:origin-center"
        >
          <slot />
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
