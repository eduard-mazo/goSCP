<script setup lang="ts">
import { ref } from 'vue'
import { LockKeyhole, Loader2, ShieldCheck, Eye, EyeOff, ArrowRight } from 'lucide-vue-next'
import { api, setToken } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import ThemeToggle from '@/components/ThemeToggle.vue'

const emit = defineEmits<{ (e: 'authenticated'): void }>()

const token = ref('')
const error = ref('')
const loading = ref(false)
const reveal = ref(false)

async function submit() {
  error.value = ''
  if (!token.value.trim()) {
    error.value = 'Please enter an access token.'
    return
  }
  loading.value = true
  setToken(token.value.trim())
  try {
    if (await api.checkAuth()) {
      emit('authenticated')
    } else {
      error.value = 'Invalid token. Check the value printed in the server console.'
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Connection failed.'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="relative flex min-h-[100dvh] items-center justify-center p-4">
    <div class="absolute right-4 top-4">
      <ThemeToggle />
    </div>

    <div class="w-full max-w-sm animate-scale-in">
      <!-- Brand mark -->
      <div class="mb-7 flex flex-col items-center text-center">
        <div class="relative mb-4 flex h-14 w-14 items-center justify-center rounded-2xl border border-primary/30 bg-primary/10">
          <LockKeyhole class="h-7 w-7 text-primary" />
          <span class="absolute inset-0 -z-10 rounded-2xl bg-primary/20 blur-xl" />
        </div>
        <h1 class="font-mono text-2xl font-semibold tracking-tight">
          goSCP
        </h1>
        <p class="mt-1 label-mono text-muted-foreground">
          transfer console
        </p>
      </div>

      <!-- Auth panel -->
      <div class="rounded-2xl border bg-card/80 p-6 shadow-xl backdrop-blur">
        <form
          class="space-y-4"
          @submit.prevent="submit"
        >
          <div class="space-y-2">
            <label
              for="token"
              class="label-mono text-muted-foreground"
            >Access token</label>
            <div class="relative">
              <Input
                id="token"
                v-model="token"
                :type="reveal ? 'text' : 'password'"
                placeholder="bearer token…"
                autofocus
                class="h-11 pr-10 font-mono"
              />
              <button
                type="button"
                class="absolute inset-y-0 right-0 flex w-10 items-center justify-center text-muted-foreground transition-colors hover:text-foreground"
                :aria-label="reveal ? 'Hide token' : 'Show token'"
                @click="reveal = !reveal"
              >
                <EyeOff
                  v-if="reveal"
                  class="h-4 w-4"
                />
                <Eye
                  v-else
                  class="h-4 w-4"
                />
              </button>
            </div>
          </div>

          <Transition
            enter-active-class="transition duration-200"
            enter-from-class="opacity-0 -translate-y-1"
          >
            <p
              v-if="error"
              class="rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
            >
              {{ error }}
            </p>
          </Transition>

          <Button
            type="submit"
            class="h-11 w-full text-base"
            :disabled="loading"
          >
            <Loader2
              v-if="loading"
              class="animate-spin"
            />
            <template v-else>
              <span>Unlock</span>
              <ArrowRight class="transition-transform group-hover:translate-x-0.5" />
            </template>
          </Button>
        </form>
      </div>

      <p class="mt-5 flex items-center justify-center gap-1.5 text-center text-xs text-muted-foreground">
        <ShieldCheck class="h-3.5 w-3.5 text-[hsl(var(--success))]" />
        The token is printed in the server console at startup.
      </p>
    </div>
  </div>
</template>
