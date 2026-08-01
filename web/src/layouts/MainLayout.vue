<template>
  <q-layout view="lHh Lpr lFf">
    <q-header elevated class="bg-primary text-white">
      <q-toolbar>
        <q-btn flat dense round icon="menu" aria-label="Menu" @click="leftDrawerOpen = !leftDrawerOpen" />
        <q-icon name="hub" size="sm" class="q-mx-sm" />
        <q-toolbar-title>mini-acm</q-toolbar-title>
        <q-chip square dense color="white" text-color="primary" class="q-mr-sm">{{ versionLabel }}</q-chip>
      </q-toolbar>
      <div class="warning-banner text-center q-py-xs text-caption">
        <q-icon name="science" size="xs" class="q-mb-xs" />
        LAB / TEST / DEV ONLY — virtual rack for ACM lifecycle demos
        <q-icon name="science" size="xs" class="q-mb-xs" />
      </div>
    </q-header>

    <q-drawer v-model="leftDrawerOpen" show-if-above bordered>
      <q-list>
        <q-item-label header>Navigation</q-item-label>
        <q-item clickable v-ripple :to="{ name: 'home' }" exact active-class="text-primary bg-grey-2">
          <q-item-section avatar><q-icon name="home" /></q-item-section>
          <q-item-section>
            <q-item-label>Home</q-item-label>
            <q-item-label caption>Overview &amp; actions</q-item-label>
          </q-item-section>
        </q-item>
        <q-item clickable v-ripple :to="{ name: 'mockups' }" active-class="text-primary bg-grey-2">
          <q-item-section avatar><q-icon name="account_tree" /></q-item-section>
          <q-item-section>
            <q-item-label>MockUps</q-item-label>
            <q-item-label caption>Lab rack blueprints</q-item-label>
          </q-item-section>
        </q-item>
        <q-item clickable v-ripple :to="{ name: 'inventory' }" active-class="text-primary bg-grey-2">
          <q-item-section avatar><q-icon name="dns" /></q-item-section>
          <q-item-section>
            <q-item-label>Inventory</q-item-label>
            <q-item-label caption>MACHINE-HOST targets</q-item-label>
          </q-item-section>
        </q-item>

        <q-separator class="q-my-md" />
        <div class="q-px-md q-pb-md text-caption text-grey-7">
          Inventory hosts power orchestration. MockUp topology paints the rack; link a host to probe &amp; plan.
        </div>
      </q-list>
    </q-drawer>

    <q-page-container>
      <router-view />
    </q-page-container>
  </q-layout>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { getHealth } from 'src/services/api'

const leftDrawerOpen = ref(false)
const versionLabel = ref('…')

onMounted(async () => {
  try {
    const h = await getHealth()
    versionLabel.value = h.version || 'dev'
  } catch {
    versionLabel.value = 'offline'
  }
})
</script>
