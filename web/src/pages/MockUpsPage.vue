<template>
  <q-page padding>
    <div class="row items-center q-mb-md">
      <div class="text-h4">MockUps</div>
      <q-space />
      <q-btn color="primary" icon="add" label="New MockUp" class="q-mr-sm" @click="openCreate" />
      <q-btn flat dense icon="refresh" label="Refresh" @click="load" />
    </div>
    <p class="text-body2 text-grey-7 q-mb-lg">
      Blueprints by genre — <strong>Validate</strong> → <strong>Deploy</strong> against a green Inventory host.
      Wizard collects gaps; Topology edits the rack.
    </p>

    <div v-if="loading" class="row justify-center q-my-xl">
      <q-spinner color="primary" size="3em" />
    </div>

    <q-banner v-else-if="error" class="bg-orange-1 text-orange-9 q-mb-lg" rounded>
      <template #avatar><q-icon name="cloud_off" color="orange-9" /></template>
      Could not reach the API ({{ error }}). Run <code>mock-me serve</code> on :8080.
    </q-banner>

    <div v-else-if="mockups.length === 0" class="text-center text-grey-7 q-my-xl">
      <q-icon name="account_tree" size="3em" class="q-mb-sm" />
      <div class="q-mb-md">No MockUps yet — create one to begin.</div>
      <q-btn color="primary" unelevated icon="add" label="New MockUp" @click="openCreate" />
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
              {{ styleLabel(m) }}
            </div>
            <div class="text-caption text-grey-6">
              {{ m.spec?.baseDomain }} · {{ m.spec?.provider }}
              <template v-if="isACMMultiCluster(m)">
                · OCP-MGMT + ACM + {{ (m.spec?.clusters || []).length }} OCP-DEPLOY
              </template>
              <template v-else-if="isSingleSNO(m)">
                · OCP-MGMT (SNO) only
              </template>
            </div>
            <div v-if="m.status?.message" class="text-caption q-mt-xs" :class="{ 'text-negative': isFailed(m) }">{{ m.status.message }}</div>
          </q-card-section>
          <q-separator />
          <q-card-actions align="right" class="q-gutter-xs">
            <q-btn flat dense size="sm" color="primary" label="Topology"
              @click="$router.push({ name: 'topology', params: { id: m.metadata.id } })" />
            <q-btn flat dense size="sm" color="primary" label="Wizard"
              :disable="isLocked(m)"
              @click="$router.push({ name: 'wizard', params: { id: m.metadata.id } })" />
            <q-btn flat dense size="sm" color="primary" label="Derive"
              :loading="deriveBusy === m.metadata.id"
              :disable="isLocked(m) || phaseBusy(m)"
              @click="doDerive(m)" />
            <q-btn flat dense size="sm" color="deep-purple-7" label="Validate"
              :loading="validateBusy === m.metadata.id"
              :disable="isLocked(m) || phaseBusy(m)"
              @click="doValidate(m)" />
            <q-btn flat dense size="sm" color="orange-9" label="Deploy"
              :loading="deployBusy === m.metadata.id"
              :disable="isLocked(m) || phaseBusy(m) || !canDeploy(m)"
              @click="doDeploy(m)" />
            <q-btn
              v-if="isFailed(m) || isDeploying(m)"
              flat dense size="sm" color="warning" label="Clean"
              :loading="cleanBusy === m.metadata.id"
              @click="doClean(m)"
            />
            <q-btn flat dense size="sm" color="negative" label="Delete" @click="doDelete(m)" />
          </q-card-actions>
        </q-card>
      </div>
    </div>

    <CreateMockUpDialog
      v-model="createOpen"
      :catalog="catalog"
      @created="load"
    />

    <DeployAssemblyDialog
      v-model="deployOpen"
      :mockup-id="deployTarget?.metadata?.id || ''"
      :mockup-name="deployTarget?.metadata?.name || ''"
      :initial-job="deployJob"
      @finished="onDeployFinished"
    />

    <ValidateWalkDialog
      v-model="validateOpen"
      :title="validateTarget?.metadata?.name || 'MockUp'"
      :result="validateResult"
      @closed="load"
    />
  </q-page>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { Dialog, Notify } from 'quasar'
import {
  listMockups, deleteMockup, deriveMockup, validateMockup, deployMockup, cleanMockup, getCatalog,
} from 'src/services/api'
import CreateMockUpDialog from 'src/components/CreateMockUpDialog.vue'
import DeployAssemblyDialog from 'src/components/DeployAssemblyDialog.vue'
import ValidateWalkDialog from 'src/components/ValidateWalkDialog.vue'

const mockups = ref([])
const catalog = ref({ genres: [], styles: [] })
const loading = ref(true)
const error = ref('')
const createOpen = ref(false)
const deriveBusy = ref('')
const validateBusy = ref('')
const deployBusy = ref('')
const cleanBusy = ref('')
const deployOpen = ref(false)
const deployTarget = ref(null)
const deployJob = ref(null)
const validateOpen = ref(false)
const validateTarget = ref(null)
const validateResult = ref(null)

function isACMMultiCluster(m) {
  return !m.spec?.style || m.spec.style === 'acm-multi-cluster'
}

function isSingleSNO(m) {
  return m.spec?.style === 'single-sno-ocp'
}

function styleLabel(m) {
  const genre = (catalog.value.genres || []).find((g) => g.id === (m.spec?.genre || 'cluster-management'))
  const style = (catalog.value.styles || []).find((s) => s.id === (m.spec?.style || 'acm-multi-cluster'))
  const g = genre?.label || m.spec?.genre || 'Cluster Management'
  const st = style?.label || m.spec?.style || 'ACM Multi-Cluster'
  return `${g} · ${st}`
}

function statusColor(phase) {
  return {
    created: 'grey-6',
    configured: 'blue-7',
    validated: 'deep-purple-6',
    deploying: 'orange-8',
    deployed: 'positive',
    failed: 'negative',
    'hub-ready': 'blue-7',
    'acm-ready': 'teal-7',
    clustered: 'orange-7',
    ready: 'positive',
  }[phase] || 'grey-6'
}

function phaseBusy(m) {
  const id = m.metadata.id
  return deriveBusy.value === id || validateBusy.value === id || deployBusy.value === id || cleanBusy.value === id
}

function isFailed(m) {
  return (m.status?.phase || '') === 'failed'
}

function isDeploying(m) {
  return (m.status?.phase || '') === 'deploying'
}

function isLocked(m) {
  return isFailed(m) || isDeploying(m)
}

function canDeploy(m) {
  const p = m.status?.phase || 'created'
  return ['configured', 'validated', 'deployed', 'hub-ready', 'acm-ready', 'created'].includes(p)
}

function openCreate() {
  createOpen.value = true
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [list, cat] = await Promise.all([listMockups(), getCatalog().catch(() => ({ genres: [], styles: [] }))])
    mockups.value = list || []
    catalog.value = cat || { genres: [], styles: [] }
  } catch (e) {
    error.value = e.message
    mockups.value = []
  } finally {
    loading.value = false
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

async function doValidate(m) {
  validateBusy.value = m.metadata.id
  validateTarget.value = m
  validateResult.value = null
  try {
    const res = await validateMockup(m.metadata.id)
    validateResult.value = res
    validateOpen.value = true
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    validateBusy.value = ''
  }
}

async function doDeploy(m) {
  deployBusy.value = m.metadata.id
  deployTarget.value = m
  deployJob.value = null
  try {
    const res = await deployMockup(m.metadata.id)
    deployJob.value = res.job
    deployOpen.value = true
    if (res.mockup) {
      // optimistic phase bump
      const idx = mockups.value.findIndex((x) => x.metadata.id === m.metadata.id)
      if (idx >= 0) mockups.value[idx] = res.mockup
    }
  } catch (e) {
    const data = e.response?.data
    const msg = typeof data === 'string' ? data : (data?.error || data?.message || e.message)
    Notify.create({ type: 'negative', message: msg, timeout: 7000 })
    await load()
  } finally {
    deployBusy.value = ''
  }
}

async function onDeployFinished() {
  await load()
}

async function doClean(m) {
  cleanBusy.value = m.metadata.id
  try {
    const res = await cleanMockup(m.metadata.id)
    Notify.create({
      type: 'positive',
      message: res.message || 'Cleaned — Validate/Deploy unlocked',
      timeout: 5000,
    })
    await load()
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    cleanBusy.value = ''
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
