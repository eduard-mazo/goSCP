<script setup lang="ts">
import { ref } from 'vue'
import { LockKeyhole, Loader2 } from 'lucide-vue-next'
import { api, setToken } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle,
} from '@/components/ui/card'

const emit = defineEmits<{ (e: 'authenticated'): void }>()

const token = ref('')
const error = ref('')
const loading = ref(false)

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
  <div class="flex min-h-screen items-center justify-center bg-background p-4">
    <Card class="w-full max-w-sm">
      <CardHeader class="text-center">
        <div class="mx-auto mb-2 flex h-12 w-12 items-center justify-center rounded-full bg-primary/10">
          <LockKeyhole class="h-6 w-6 text-primary" />
        </div>
        <CardTitle class="text-2xl">goSCP</CardTitle>
        <CardDescription>Enter your access token to exchange files</CardDescription>
      </CardHeader>
      <CardContent class="space-y-3">
        <form class="space-y-3" @submit.prevent="submit">
          <Input
            v-model="token"
            type="password"
            placeholder="Bearer token"
            autofocus
          />
          <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
          <Button type="submit" class="w-full" :disabled="loading">
            <Loader2 v-if="loading" class="animate-spin" />
            <span>{{ loading ? 'Verifying…' : 'Unlock' }}</span>
          </Button>
        </form>
      </CardContent>
      <CardFooter class="justify-center">
        <p class="text-xs text-muted-foreground">
          The token is shown in the server console at startup.
        </p>
      </CardFooter>
    </Card>
  </div>
</template>
