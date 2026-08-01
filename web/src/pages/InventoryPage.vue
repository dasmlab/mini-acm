<template>
  <q-page padding class="wide">
    <div class="row items-center q-mb-md">
      <div class="col">
        <div class="text-h5">Inventory</div>
        <div class="text-caption text-grey-6">
          MACHINE-HOST targets for orchestration — SSH endpoint + identity. Seed is dasm@192.168.1.142.
        </div>
      </div>
      <q-btn color="primary" icon="add" label="Add host" @click="openCreate" />
    </div>

    <div v-if="loading" class="row justify-center q-my-xl"><q-spinner size="3em" color="primary" /></div>

    <q-list v-else bordered class="rounded-borders bg-white">
      <q-item v-for="h in hosts" :key="h.id" class="q-py-md">
        <q-item-section avatar>
          <q-avatar :color="statusColor(h.status)" text-color="white" icon="dns" />
        </q-item-section>
        <q-item-section>
          <q-item-label class="text-weight-medium">
            {{ h.name }}
            <q-badge v-if="h.seed" color="primary" outline class="q-ml-sm">SEED</q-badge>
            <q-badge :color="statusColor(h.status)" class="q-ml-sm">{{ h.status }}</q-badge>
          </q-item-label>
          <q-item-label caption>
            {{ h.sshUser }}@{{ h.sshHost }}:{{ h.sshPort || 22 }}
            <span v-if="h.identityFile"> · key {{ h.identityFile }}</span>
          </q-item-label>
          <q-item-label caption v-if="h.statusMessage" class="q-mt-xs">{{ h.statusMessage }}</q-item-label>
          <div v-if="h.facts && Object.keys(h.facts).length" class="q-mt-sm text-caption text-grey-7">
            <span v-for="(v, k) in h.facts" :key="k" class="q-mr-md"><b>{{ k }}</b>: {{ v }}</span>
          </div>
        </q-item-section>
        <q-item-section side>
          <div class="column q-gutter-sm">
            <q-btn
              unelevated color="primary" icon="sensors" label="Probe"
              :loading="probing === h.id"
              @click="onProbe(h)"
            />
            <q-btn flat dense color="grey-8" icon="edit" label="Edit" @click="openEdit(h)" />
            <q-btn
              flat dense color="negative" icon="delete" label="Remove"
              :disable="!!h.seed"
              @click="onDelete(h)"
            />
          </div>
        </q-item-section>
      </q-item>
      <q-item v-if="!hosts.length">
        <q-item-section class="text-grey-6">No hosts yet — seed should appear on first load.</q-item-section>
      </q-item>
    </q-list>

    <q-banner class="q-mt-lg bg-grey-2 rounded-borders" dense>
      <template #avatar><q-icon name="info" color="primary" /></template>
      Probe uses the private key path (never stored as key material). From this lab box SSH to
      <code>dasm@192.168.1.142</code> works; libvirt may still need install/start before deploy.
      K8s as-a-service needs network + mounted identity (<code>INVENTORY_SSH_KEY</code>) to probe LAN hosts.
    </q-banner>

    <q-dialog v-model="formOpen" persistent>
      <q-card style="min-width: 420px">
        <q-card-section>
          <div class="text-h6">{{ editing ? 'Edit host' : 'Add MACHINE-HOST' }}</div>
        </q-card-section>
        <q-card-section class="q-pt-none">
          <q-input v-model="form.name" outlined dense label="Name" class="q-mb-sm" />
          <q-input v-model="form.sshUser" outlined dense label="SSH user" class="q-mb-sm" />
          <q-input v-model="form.sshHost" outlined dense label="SSH host / IP" class="q-mb-sm"
            hint="e.g. 192.168.1.142 — keys already exchanged" />
          <q-input v-model.number="form.sshPort" type="number" outlined dense label="SSH port" class="q-mb-sm" />
          <q-input v-model="form.identityFile" outlined dense label="Identity file (private key path)" class="q-mb-sm"
            hint="~/.ssh/id_ecdsa or INVENTORY_SSH_KEY" />
          <q-input v-model="form.notes" outlined dense type="textarea" autogrow label="Notes" />
        </q-card-section>
        <q-card-actions align="right">
          <q-btn flat label="Cancel" v-close-popup />
          <q-btn color="primary" :label="editing ? 'Save' : 'Add'" :loading="saving" @click="onSaveForm" />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { Dialog, Notify } from 'quasar'
import {
  listInventory, createInventory, updateInventory, deleteInventory, probeInventory,
} from 'src/services/api'

const loading = ref(true)
const saving = ref(false)
const probing = ref(null)
const hosts = ref([])
const formOpen = ref(false)
const editing = ref(null)
const form = ref(emptyForm())

function emptyForm() {
  return {
    name: '',
    sshUser: 'dasm',
    sshHost: '',
    sshPort: 22,
    identityFile: '~/.ssh/id_ecdsa',
    notes: '',
  }
}

function statusColor(s) {
  if (s === 'reachable') return 'positive'
  if (s === 'partial') return 'warning'
  if (s === 'unreachable') return 'negative'
  return 'grey'
}

async function load() {
  loading.value = true
  try {
    hosts.value = await listInventory()
  } catch (e) {
    Notify.create({ type: 'negative', message: e.message })
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  form.value = emptyForm()
  formOpen.value = true
}

function openEdit(h) {
  editing.value = h
  form.value = {
    name: h.name,
    sshUser: h.sshUser,
    sshHost: h.sshHost,
    sshPort: h.sshPort || 22,
    identityFile: h.identityFile || '',
    notes: h.notes || '',
  }
  formOpen.value = true
}

async function onSaveForm() {
  saving.value = true
  try {
    if (editing.value) {
      await updateInventory(editing.value.id, {
        ...editing.value,
        ...form.value,
      })
      Notify.create({ type: 'positive', message: 'Updated.' })
    } else {
      await createInventory(form.value)
      Notify.create({ type: 'positive', message: 'Host added.' })
    }
    formOpen.value = false
    await load()
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    saving.value = false
  }
}

async function onProbe(h) {
  probing.value = h.id
  try {
    const res = await probeInventory(h.id)
    Notify.create({
      type: res.authOK ? (res.libvirtReady ? 'positive' : 'warning') : 'negative',
      message: res.message,
      timeout: 7000,
    })
    await load()
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    probing.value = null
  }
}

function onDelete(h) {
  Dialog.create({
    title: 'Remove inventory host?',
    message: `${h.name} (${h.sshUser}@${h.sshHost})`,
    cancel: true,
  }).onOk(async () => {
    try {
      await deleteInventory(h.id)
      Notify.create({ type: 'positive', message: 'Removed.' })
      await load()
    } catch (e) {
      Notify.create({ type: 'negative', message: e.response?.data || e.message })
    }
  })
}

onMounted(load)
</script>

<style scoped>
.wide { max-width: 1100px; margin: 0 auto; }
</style>
