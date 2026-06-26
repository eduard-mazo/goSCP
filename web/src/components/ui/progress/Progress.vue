<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<{
    /** 0..1 */
    value?: number
    /** Render as an indeterminate animated bar. */
    indeterminate?: boolean
    class?: string
  }>(),
  { value: 0, indeterminate: false },
)

const pct = computed(() => Math.round(Math.min(1, Math.max(0, props.value)) * 100))
</script>

<template>
  <div
    role="progressbar"
    :aria-valuenow="indeterminate ? undefined : pct"
    aria-valuemin="0"
    aria-valuemax="100"
    :class="cn('h-1.5 w-full overflow-hidden rounded-full bg-muted', props.class)"
  >
    <div
      class="h-full rounded-full bg-primary transition-[width] duration-200 ease-out"
      :class="indeterminate && 'w-1/3 animate-[row-in_1.2s_ease-in-out_infinite_alternate]'"
      :style="!indeterminate ? { width: pct + '%' } : undefined"
    />
  </div>
</template>
