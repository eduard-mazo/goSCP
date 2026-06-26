<script setup lang="ts">
import { ref, computed } from 'vue'
import { LockKeyhole, Loader2, ShieldCheck, Eye, EyeOff, ArrowRight } from 'lucide-vue-next'
import { api, setToken, ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import ThemeToggle from '@/components/ThemeToggle.vue'

const emit = defineEmits<{ (e: 'authenticated'): void }>()

// 'password' exchanges a login password for a token via the API; 'token' pastes
// a raw bearer token (the value printed in the server console).
const mode = ref<'password' | 'token'>('password')
const password = ref('')
const token = ref('')
const error = ref('')
const loading = ref(false)
const reveal = ref(false)

const isPassword = computed(() => mode.value === 'password')

function toggleMode() {
  mode.value = isPassword.value ? 'token' : 'password'
  error.value = ''
  reveal.value = false
}

async function submit() {
  error.value = ''
  if (isPassword.value && !password.value) {
    error.value = 'Please enter the password.'
    return
  }
  if (!isPassword.value && !token.value.trim()) {
    error.value = 'Please enter an access token.'
    return
  }
  loading.value = true
  try {
    if (isPassword.value) {
      setToken(await api.requestToken(password.value))
    } else {
      setToken(token.value.trim())
    }
    if (await api.checkAuth()) {
      emit('authenticated')
    } else {
      error.value = 'Authentication failed. Check your credentials.'
    }
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      error.value = isPassword.value ? 'Incorrect password.' : 'Invalid access token.'
    } else if (e instanceof ApiError && e.status === 404) {
      error.value = 'Password login is not enabled — use an access token instead.'
    } else {
      error.value = e instanceof Error ? e.message : 'Connection failed.'
    }
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
              :for="isPassword ? 'password' : 'token'"
              class="label-mono text-muted-foreground"
            >{{ isPassword ? 'Password' : 'Access token' }}</label>
            <div class="relative">
              <Input
                v-if="isPassword"
                id="password"
                v-model="password"
                :type="reveal ? 'text' : 'password'"
                placeholder="password…"
                autofocus
                class="h-11 pr-10 font-mono"
              />
              <Input
                v-else
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
                :aria-label="reveal ? 'Hide' : 'Show'"
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

          <button
            type="button"
            class="w-full text-center text-xs text-muted-foreground transition-colors hover:text-foreground"
            @click="toggleMode"
          >
            {{ isPassword ? 'Use an access token instead' : 'Use a password instead' }}
          </button>
        </form>
      </div>

      <p class="mt-5 flex items-center justify-center gap-1.5 text-center text-xs text-muted-foreground">
        <ShieldCheck class="h-3.5 w-3.5 text-[hsl(var(--success))]" />
        {{ isPassword ? 'Sign in with the password set on the server.' : 'The token is printed in the server console at startup.' }}
      </p>
    </div>
  </div>
</template>
