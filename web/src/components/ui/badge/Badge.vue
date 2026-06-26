<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'outline' | 'mono' | 'success'
    class?: string
  }>(),
  { variant: 'default' },
)

const variants: Record<string, string> = {
  default: 'border-transparent bg-secondary text-secondary-foreground',
  outline: 'border-border text-muted-foreground',
  mono: 'border-border bg-muted/50 font-mono text-muted-foreground tracking-tight',
  success: 'border-transparent bg-[hsl(var(--success)/0.15)] text-[hsl(var(--success))]',
}

const classes = computed(() =>
  cn(
    'inline-flex items-center gap-1 rounded-md border px-1.5 py-0.5 text-[0.7rem] font-medium leading-none',
    variants[props.variant],
    props.class,
  ),
)
</script>

<template>
  <span :class="classes"><slot /></span>
</template>
