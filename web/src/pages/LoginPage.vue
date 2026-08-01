<template>
  <q-page class="flex flex-center column q-pa-xl">
    <div class="text-h4 q-mb-sm">mini-mock</div>
    <p class="text-body2 text-grey-7 q-mb-lg text-center" style="max-width: 420px">
      Sign in with the dasmlab Keycloak realm to author MockUps
      (Cluster Management · ACM Multi-Cluster, and more).
    </p>
    <q-btn color="primary" unelevated size="lg" icon="login" label="Sign in with Keycloak" @click="login" />
    <p v-if="hint" class="text-caption text-negative q-mt-md">{{ hint }}</p>
  </q-page>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuth } from 'src/services/auth'

const auth = useAuth()
const route = useRoute()
const router = useRouter()
const hint = ref('')

function login() {
  auth.login()
}

onMounted(async () => {
  await auth.init()
  if (!auth.authEnabled.value) {
    router.replace(route.query.returnTo || '/')
    return
  }
  if (auth.isAdmin.value) {
    router.replace(route.query.returnTo || '/')
  } else if (auth.isAuthenticated.value) {
    hint.value = 'Signed in but missing mini-mock client role “admin”. Ask an admin to assign it in Keycloak.'
  }
})
</script>
