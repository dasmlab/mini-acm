<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h4">MockUps</div>
      <q-space />
      <q-btn color="primary" icon="add" label="Create MockUp" class="q-mr-sm" @click="createOpen = true" />
      <q-btn flat dense icon="refresh" label="Refresh" @click="load" />
    </div>
    <p class="text-body2 text-grey-7 q-mb-lg">
      Lab rack blueprints — open Topology to place hosts, then Wizard to fill gaps.
    </p>

    <div v-if="loading" class="row justify-center q-my-xl">
      <q-spinner color="primary" size="3em" />
    </div>

    <q-banner v-else-if="error" class="bg-orange-1 text-orange-9 q-mb-lg" rounded>
      <template #avatar><q-icon name="cloud_off" color="orange-9" /></template>
      Could not reach the API ({{ error }}). Run <code>mini-acm serve</code> on :8080.
    </q-banner>

    <div v-else-if="mockups.length === 0" class="text-center text-grey-7 q-my-xl">
      <q-icon name="account_tree" size="3em" class="q-mb-sm" />
      <div>No MockUps yet — create one to begin.</div>
    </div>

    <div v-else class="row q-col-gutter-md">
      <div v-for="m in mockups" :key="m.metadata.id" class="col-12 col-sm-6 col-md-4">
        <q-card flat bordered>
          <q-card-section>
            <div class="row items-center no-wrap">
              <div class="text-h6 ellipsis">{{ m.metadata.name }}</div>
              <q-space />
              <q-badge :color="statusColor(m.status?.phase)" class="text-capitalize">
                {{ m.status?.phase || 'created' }}
              </q-badge>
            </div>
            <div class="text-caption text-grey-7 q-mt-xs">
              {{ m.spec?.baseDomain }} · {{ m.spec?.provider }}
            </div>
            <div class="text-caption text-grey-6">
              hub + ACM + {{ (m.spec?.clusters || []).length }} cluster(s)
            </div>
            <div v-if="m.status?.message" class="text-caption q-mt-xs">{{ m.status.message }}</div>
          </q-card-section>
          <q-separator />
          <q-card-actions align="right" class="q-gutter-xs">
            <q-btn flat dense size="sm" color="primary" label="Topology"
              @click="$router.push({ name: 'topology', params: { id: m.metadata.id } })" />
            <q-btn flat dense size="sm" color="primary" label="Wizard"
              @click="$router.push({ name: 'wizard', params: { id: m.metadata.id } })" />
            <q-btn flat dense size="sm" color="primary" label="Derive"
              :loading="deriveBusy === m.metadata.id" @click="doDerive(m)" />
            <q-btn flat dense size="sm" color="negative" label="Delete" @click="doDelete(m)" />
          </q-card-actions>
        </q-card>
      </div>
    </div>

    <q-dialog v-model="createOpen">
      <q-card style="min-width: 420px">
        <q-card-section class="text-h6">Create MockUp</q-card-section>
        <q-card-section>
          <q-input v-model="form.name" filled label="Name" class="q-mb-md" hint="e.g. lab-rack-1" />
          <q-input v-model="form.baseDomain" filled label="Base domain" class="q-mb-md" />
          <q-select v-model="form.provider" :options="['libvirt']" filled label="Provider" class="q-mb-md" />
          <q-input v-model="form.notes" filled label="Notes" type="textarea" autogrow />
        </q-card-section>
        <q-card-actions align="right">
          <q-btn flat label="Cancel" v-close-popup />
          <q-btn color="primary" label="Create" :loading="creating" @click="doCreate" />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { Dialog, Notify } from 'quasar'
import { listMockups, createMockup, deleteMockup, deriveMockup } from 'src/services/api'

const mockups = ref([])
const loading = ref(true)
const error = ref('')
const createOpen = ref(false)
const creating = ref(false)
const deriveBusy = ref('')
const form = reactive({
  name: 'lab-rack-1',
  baseDomain: 'lab.example.net',
  provider: 'libvirt',
  notes: '',
})

function statusColor(phase) {
  return {
    created: 'grey-6',
    configured: 'blue-7',
    'hub-ready': 'blue-7',
    'acm-ready': 'teal-7',
    clustered: 'orange-7',
    ready: 'positive',
  }[phase] || 'grey-6'
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    mockups.value = (await listMockups()) || []
  } catch (e) {
    error.value = e.message
    mockups.value = []
  } finally {
    loading.value = false
  }
}

async function doCreate() {
  creating.value = true
  try {
    await createMockup({ ...form })
    createOpen.value = false
    Notify.create({ type: 'positive', message: 'MockUp created.' })
    await load()
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    creating.value = false
  }
}

async function doDerive(m) {
  deriveBusy.value = m.metadata.id
  try {
    const res = await deriveMockup(m.metadata.id)
    Notify.create({
      type: 'positive',
      message: `Derived: ${Object.values(res.paths || {}).join(', ')}`,
      timeout: 5000,
    })
    await load()
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    deriveBusy.value = ''
  }
}

function doDelete(m) {
  Dialog.create({
    title: 'Delete MockUp?',
    message: `Removes ${m.metadata.name} (${m.metadata.id}) from data/mockups.`,
    cancel: true,
    persistent: true,
  }).onOk(async () => {
    try {
      await deleteMockup(m.metadata.id)
      Notify.create({ type: 'positive', message: 'Deleted.' })
      await load()
    } catch (e) {
      Notify.create({ type: 'negative', message: e.response?.data || e.message })
    }
  })
}

onMounted(load)
</script>
