<template>
  <q-page class="mockups-page" padding>
    <header class="page-head">
      <div>
        <p class="page-kicker">Lab rack blueprints</p>
        <h1 class="page-title">MockUps</h1>
        <p class="page-lead">
          Validate → Deploy against a green Inventory host. Wizard collects gaps; Topology edits the rack.
        </p>
      </div>
      <div class="page-head-actions">
        <q-btn color="primary" unelevated icon="add" label="New MockUp" @click="openCreate" />
        <q-btn outline color="primary" icon="refresh" label="Refresh" :loading="loading" @click="load" />
      </div>
    </header>

    <div v-if="loading && mockups.length === 0" class="row justify-center q-my-xl">
      <q-spinner color="primary" size="3em" />
    </div>

    <q-banner v-else-if="error" class="bg-orange-2 text-orange-10 q-mb-lg" rounded>
      <template #avatar><q-icon name="cloud_off" color="orange-9" /></template>
      Could not reach the API ({{ error }}). Run <code>mock-me serve</code> on :8080.
    </q-banner>

    <div v-else-if="mockups.length === 0" class="empty-state">
      <q-icon name="account_tree" size="56px" color="blue-grey-5" />
      <div class="empty-title">No MockUps yet</div>
      <div class="empty-sub">Create a blueprint, then Validate → Deploy on a ready MACHINE-HOST.</div>
      <q-btn color="primary" unelevated icon="add" label="New MockUp" class="q-mt-md" @click="openCreate" />
    </div>

    <div v-else class="mockup-grid">
      <article
        v-for="m in mockups"
        :key="m.metadata.id"
        class="mockup-card"
        :class="[`phase-${m.status?.phase || 'created'}`, { locked: isLocked(m) }]"
      >
        <div class="card-accent" />
        <div class="card-body">
          <div class="card-top">
            <h2 class="card-name">{{ m.metadata.name }}</h2>
            <q-badge
              :color="statusColor(m.status?.phase)"
              class="phase-badge text-capitalize"
              :outline="false"
            >
              {{ m.status?.phase || 'created' }}
            </q-badge>
          </div>

          <div class="card-style">{{ styleLabel(m) }}</div>

          <div class="card-meta">
            <span>{{ m.spec?.baseDomain }}</span>
            <span class="dot">·</span>
            <span>{{ m.spec?.provider }}</span>
            <template v-if="isACMMultiCluster(m)">
              <span class="dot">·</span>
              <span>OCP-MGMT + ACM + {{ (m.spec?.clusters || []).length }} OCP-DEPLOY</span>
            </template>
            <template v-else-if="isSingleSNO(m)">
              <span class="dot">·</span>
              <span>OCP-MGMT (SNO)</span>
            </template>
          </div>

          <div
            v-if="m.status?.message"
            class="card-status"
            :class="{ danger: isFailed(m), warn: isDeploying(m) }"
          >
            {{ m.status.message }}
          </div>

          <div v-if="eeBlockReason && !isLocked(m)" class="card-status warn">
            Deploy blocked — {{ eeBlockReason }}
            <router-link class="inv-link" :to="{ name: 'inventory' }">Open Inventory</router-link>
          </div>

          <div class="card-actions">
            <q-btn
              unelevated
              color="primary"
              size="sm"
              icon="account_tree"
              label="Topology"
              @click="$router.push({ name: 'topology', params: { id: m.metadata.id } })"
            />
            <q-btn
              outline
              color="primary"
              size="sm"
              label="Wizard"
              :disable="isLocked(m)"
              @click="$router.push({ name: 'wizard', params: { id: m.metadata.id } })"
            />
            <q-btn
              flat
              color="blue-grey-8"
              size="sm"
              label="Derive"
              :loading="deriveBusy === m.metadata.id"
              :disable="isLocked(m) || phaseBusy(m)"
              @click="doDerive(m)"
            />
            <q-btn
              outline
              color="deep-purple-7"
              size="sm"
              label="Validate"
              :loading="validateBusy === m.metadata.id"
              :disable="isLocked(m) || phaseBusy(m)"
              @click="doValidate(m)"
            />
            <q-btn
              unelevated
              color="orange-9"
              size="sm"
              icon="rocket_launch"
              label="Deploy"
              :loading="deployBusy === m.metadata.id"
              :disable="isLocked(m) || phaseBusy(m) || !canDeploy(m)"
              @click="doDeploy(m)"
            >
              <q-tooltip v-if="eeBlockReason">{{ eeBlockReason }}</q-tooltip>
            </q-btn>
            <q-space />
            <q-btn
              v-if="showClean(m)"
              unelevated
              color="amber-9"
              text-color="dark"
              size="sm"
              icon="cleaning_services"
              label="Clean"
              :loading="cleanBusy === m.metadata.id"
              @click="doClean(m)"
            >
              <q-tooltip>Reset deploy state so Validate/Deploy can run again (leaves host VMs in place)</q-tooltip>
            </q-btn>
            <q-btn
              flat
              color="negative"
              size="sm"
              icon="delete"
              label="Delete"
              @click="doDelete(m)"
            />
          </div>
        </div>
      </article>
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
import { computed, onMounted, ref } from 'vue'
import { Dialog, Notify } from 'quasar'
import {
  listMockups, deleteMockup, deriveMockup, validateMockup, deployMockup, cleanMockup, getCatalog, listInventory,
} from 'src/services/api'
import CreateMockUpDialog from 'src/components/CreateMockUpDialog.vue'
import DeployAssemblyDialog from 'src/components/DeployAssemblyDialog.vue'
import ValidateWalkDialog from 'src/components/ValidateWalkDialog.vue'

const mockups = ref([])
const catalog = ref({ genres: [], styles: [] })
const inventory = ref([])
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

/** Prefer seed / ready host — same resolution spirit as the API. */
const deployInventoryHost = computed(() => {
  const list = inventory.value || []
  return list.find((h) => h.seed)
    || list.find((h) => h.status === 'reachable')
    || list[0]
    || null
})

/** Why Deploy cannot start (Inventory EE prereqs). Empty = OK. */
const eeBlockReason = computed(() => {
  const h = deployInventoryHost.value
  if (!h) return 'No Inventory MACHINE-HOST — add and Probe one first'
  if (h.status === 'unreachable') return `${h.name} is unreachable — Probe Inventory`
  if (h.status === 'partial') return `${h.name} is partial (libvirt) — Fix this on Inventory`
  const oi = String(h.facts?.openshiftInstall || '').trim()
  const ee = String(h.facts?.mockMeEE || '').trim()
  if (ee === 'ready' || (oi && oi !== 'missing')) {
    // EE or legacy host installer present
  } else {
    return `curated mock-me-ee missing on ${h.name} — Probe → Fix this (ensure-mock-me-ee)`
  }
  const pod = String(h.facts?.podman || '').trim().toLowerCase()
  if (!pod || pod === 'missing' || pod.includes('missing')) {
    return `podman missing on ${h.name} — Fix this on Inventory`
  }
  return ''
})

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
    created: 'blue-grey-6',
    configured: 'blue-8',
    validated: 'deep-purple-7',
    deploying: 'orange-9',
    deployed: 'positive',
    failed: 'negative',
    'hub-ready': 'blue-8',
    'acm-ready': 'teal-8',
    clustered: 'orange-8',
    ready: 'positive',
  }[phase] || 'blue-grey-6'
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

function isDeployed(m) {
  return (m.status?.phase || '') === 'deployed'
}

function isLocked(m) {
  return isFailed(m) || isDeploying(m)
}

function showClean(m) {
  return isFailed(m) || isDeploying(m) || isDeployed(m)
}

function canDeploy(m) {
  const p = m.status?.phase || 'created'
  if (!['configured', 'validated', 'deployed', 'hub-ready', 'acm-ready', 'created'].includes(p)) {
    return false
  }
  return !eeBlockReason.value
}

function openCreate() {
  createOpen.value = true
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [list, cat, inv] = await Promise.all([
      listMockups(),
      getCatalog().catch(() => ({ genres: [], styles: [] })),
      listInventory().catch(() => []),
    ])
    mockups.value = list || []
    catalog.value = cat || { genres: [], styles: [] }
    inventory.value = inv || []
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

<style scoped>
.mockups-page {
  background:
    radial-gradient(ellipse 80% 50% at 10% -10%, rgba(21, 101, 192, 0.12), transparent 55%),
    radial-gradient(ellipse 60% 40% at 100% 0%, rgba(0, 131, 143, 0.08), transparent 50%),
    #eef2f5;
  min-height: calc(100vh - 100px);
  max-width: none;
  margin: 0;
  padding-bottom: 2.5rem !important;
}

.page-head {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  justify-content: space-between;
  gap: 1rem 1.5rem;
  margin-bottom: 1.5rem;
  max-width: 1280px;
  margin-left: auto;
  margin-right: auto;
}

.page-kicker {
  margin: 0 0 0.2rem;
  font-size: 0.78rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: #1565c0;
}

.page-title {
  margin: 0;
  font-size: 2rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  color: #0d2137;
  line-height: 1.15;
}

.page-lead {
  margin: 0.45rem 0 0;
  max-width: 42rem;
  color: #455a64;
  font-size: 0.95rem;
  line-height: 1.45;
}

.page-head-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.empty-state {
  text-align: center;
  padding: 3.5rem 1rem;
  max-width: 1280px;
  margin: 0 auto;
  background: #fff;
  border-radius: 14px;
  border: 1px solid #cfd8dc;
}

.empty-title {
  margin-top: 0.75rem;
  font-size: 1.2rem;
  font-weight: 700;
  color: #263238;
}

.empty-sub {
  margin-top: 0.35rem;
  color: #607d8b;
  font-size: 0.92rem;
}

.mockup-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 1.1rem;
  max-width: 1280px;
  margin: 0 auto;
}

.mockup-card {
  position: relative;
  display: flex;
  background: #fff;
  border-radius: 14px;
  border: 1px solid #b0bec5;
  box-shadow: 0 2px 0 rgba(13, 33, 55, 0.04), 0 10px 28px rgba(13, 33, 55, 0.08);
  overflow: hidden;
  transition: box-shadow 0.18s ease, transform 0.18s ease;
}

.mockup-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 0 rgba(13, 33, 55, 0.05), 0 16px 36px rgba(13, 33, 55, 0.12);
}

.mockup-card.locked {
  border-color: #ef9a9a;
}

.mockup-card.phase-failed {
  border-color: #e57373;
  background: linear-gradient(180deg, #fff8f8 0%, #fff 40%);
}

.mockup-card.phase-deploying {
  border-color: #ffb74d;
}

.mockup-card.phase-deployed {
  border-color: #81c784;
}

.mockup-card.phase-validated {
  border-color: #9575cd;
}

.card-accent {
  width: 6px;
  flex-shrink: 0;
  background: #607d8b;
}

.phase-created .card-accent { background: #546e7a; }
.phase-configured .card-accent { background: #1976d2; }
.phase-validated .card-accent { background: #5e35b1; }
.phase-deploying .card-accent { background: #ef6c00; }
.phase-deployed .card-accent { background: #2e7d32; }
.phase-failed .card-accent { background: #c62828; }

.card-body {
  flex: 1;
  min-width: 0;
  padding: 1rem 1.05rem 0.9rem;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.card-top {
  display: flex;
  align-items: flex-start;
  gap: 0.65rem;
}

.card-name {
  margin: 0;
  flex: 1;
  min-width: 0;
  font-size: 1.2rem;
  font-weight: 800;
  color: #0d2137;
  letter-spacing: -0.01em;
  line-height: 1.25;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.phase-badge {
  font-weight: 700;
  font-size: 0.72rem;
  padding: 0.28rem 0.55rem;
}

.card-style {
  font-size: 0.86rem;
  font-weight: 600;
  color: #37474f;
}

.card-meta {
  font-size: 0.78rem;
  color: #546e7a;
  line-height: 1.4;
}

.card-meta .dot {
  margin: 0 0.2rem;
  color: #90a4ae;
}

.card-status {
  margin-top: 0.35rem;
  padding: 0.55rem 0.65rem;
  border-radius: 8px;
  background: #eceff1;
  color: #37474f;
  font-size: 0.78rem;
  line-height: 1.4;
  border-left: 3px solid #78909c;
}

.card-status.danger {
  background: #ffebee;
  color: #b71c1c;
  border-left-color: #c62828;
  font-weight: 600;
}

.card-status.warn {
  background: #fff3e0;
  color: #e65100;
  border-left-color: #ef6c00;
  font-weight: 600;
}

.inv-link {
  margin-left: 0.45rem;
  color: #1565c0;
  font-weight: 700;
  text-decoration: underline;
}

.card-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
  margin-top: 0.75rem;
  padding-top: 0.75rem;
  border-top: 1px solid #cfd8dc;
}

@media (max-width: 600px) {
  .page-title { font-size: 1.65rem; }
  .mockup-grid { grid-template-columns: 1fr; }
}
</style>
