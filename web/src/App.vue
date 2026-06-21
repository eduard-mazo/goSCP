<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { api, getToken } from '@/api/client'
import LoginScreen from '@/components/LoginScreen.vue'
import FileBrowser from '@/components/FileBrowser.vue'

const authed = ref(false)
const ready = ref(false)

onMounted(async () => {
  if (getToken()) {
    try {
      authed.value = await api.checkAuth()
    } catch {
      authed.value = false
    }
  }
  ready.value = true
})
</script>

<template>
  <template v-if="ready">
    <FileBrowser v-if="authed" @logout="authed = false" />
    <LoginScreen v-else @authenticated="authed = true" />
  </template>
</template>
