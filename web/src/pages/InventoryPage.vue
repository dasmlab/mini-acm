<template>
  <q-page class="inventory-page" padding>
    <header class="page-head">
      <div>
        <p class="page-kicker">MACHINE-HOST targets</p>
        <div class="title-row">
          <h1 class="page-title">Inventory</h1>
          <q-btn flat dense round icon="help_outline" color="blue-grey-7" aria-label="About inventory">
            <q-menu anchor="bottom left" self="top left" max-width="360px">
              <div class="help-menu">
                <div class="help-title">Probe → Fix → Deploy</div>
                <p>
                  <b>unreachable</b> — no TCP/SSH.
                  <b>partial</b> — SSH OK, missing libvirt.
                  <b>libvirt ready</b> — can orchestrate guests.
                  <b>deploy blocked</b> — need podman + curated mock-me-ee.
                </p>
                <p>
                  Assume subscribed host + sudo. Fix installs libvirt/podman and pulls
                  <code>mock-me-ee</code>. Use <b>Stretched</b> when the cluster must reach the host via VPN.
                </p>
              </div>
            </q-menu>
          </q-btn>
        </div>
        <p class="page-lead">Probe readiness, Fix gaps, then Deploy from a MockUp.</p>
      </div>
      <q-btn color="primary" unelevated icon="add" label="Add host" @click="openCreate" />
    </header>

    <div v-if="loading" class="row justify-center q-my-xl">
      <q-spinner size="3em" color="primary" />
    </div>

    <div v-else-if="!hosts.length" class="empty-state">
      <q-icon name="dns" size="56px" color="blue-grey-5" />
      <div class="empty-title">No hosts yet</div>
      <div class="empty-sub">Seed host should appear on first load, or add one manually.</div>
    </div>

    <div v-else class="host-grid">
      <article
        v-for="h in hosts"
        :key="h.id"
        class="host-card"
        :class="[
          `status-${h.status || 'unknown'}`,
          { 'deploy-blocked': isDeployBlocked(h) },
        ]"
      >
        <div class="card-accent" />
        <div class="card-body">
          <div class="card-top">
            <q-avatar :color="statusColor(h)" text-color="white" :icon="statusIcon(h)" size="40px" />
            <div class="card-identity">
              <div class="name-row">
                <h2 class="card-name">{{ h.name }}</h2>
                <q-badge v-if="h.seed" color="primary" outline dense>SEED</q-badge>
              </div>
              <div class="chip-row">
                <q-badge :color="statusColor(h)">{{ statusLabel(h) }}</q-badge>
                <q-badge v-if="isDeployBlocked(h)" color="orange-9">deploy blocked</q-badge>
                <q-badge v-else-if="h.status === 'reachable'" color="teal-8">EE ok</q-badge>
              </div>
            </div>
            <div class="icon-tools">
              <q-btn
                v-if="(h.issues || []).length"
                flat
                dense
                round
                :icon="issueIcon(h)"
                :color="issueColor(h)"
                aria-label="Host issues"
              >
                <q-badge
                  v-if="h.issues.length > 1"
                  floating
                  color="orange-9"
                  :label="String(h.issues.length)"
                />
                <q-menu anchor="bottom right" self="top right" max-width="380px">
                  <div class="detail-menu">
                    <div class="help-title">Issues</div>
                    <div
                      v-for="iss in h.issues"
                      :key="iss.id"
                      class="issue-line"
                      :class="{ error: iss.severity === 'error' || iss.id === 'openshift-install-missing' }"
                    >
                      {{ iss.message }}
                      <q-badge v-if="iss.fixable" dense color="warning" outline class="q-ml-xs">fixable</q-badge>
                    </div>
                    <div v-if="isDeployBlocked(h)" class="issue-line warn q-mt-sm">
                      OCP Deploy needs curated <code>mock-me-ee</code> (openshift-install in container).
                      Use Fix this → ensure-mock-me-ee, then Probe again.
                    </div>
                  </div>
                </q-menu>
                <q-tooltip>Issues</q-tooltip>
              </q-btn>
              <q-btn flat dense round icon="help_outline" color="blue-grey-6" aria-label="Host details">
                <q-menu anchor="bottom right" self="top right" max-width="420px">
                  <div class="detail-menu">
                    <div class="help-title">Host details</div>
                    <div class="fact-line"><b>Endpoint</b> {{ h.sshUser }}@{{ effectiveHost(h) }}:{{ h.sshPort || 22 }}</div>
                    <div v-if="h.stretched && h.sshHost" class="fact-line">
                      <b>LAN</b> {{ h.sshHost }} · <b>VPN</b> {{ h.stretchedHost }}
                    </div>
                    <div v-if="h.identityFile" class="fact-line"><b>identity</b> {{ h.identityFile }}</div>
                    <div v-if="h.statusMessage" class="fact-line status-msg">{{ h.statusMessage }}</div>
                    <q-separator class="q-my-sm" />
                    <div v-if="h.facts && Object.keys(h.facts).length" class="facts-grid">
                      <div v-for="(v, k) in h.facts" :key="k" class="fact-line">
                        <b>{{ k }}</b> {{ v }}
                      </div>
                    </div>
                    <div v-else class="text-caption text-grey-6">No probe facts yet — run Probe.</div>
                  </div>
                </q-menu>
                <q-tooltip>Details</q-tooltip>
              </q-btn>
            </div>
          </div>

          <div class="card-endpoint">
            {{ h.sshUser }}@{{ effectiveHost(h) }}:{{ h.sshPort || 22 }}
            <q-badge v-if="h.stretched" color="deep-purple" outline dense class="q-ml-xs">stretched</q-badge>
          </div>

          <div class="card-actions">
            <q-btn
              unelevated
              color="primary"
              size="sm"
              icon="sensors"
              label="Probe"
              :loading="probing === h.id"
              @click="onProbe(h)"
            />
            <q-btn
              v-if="hasFixable(h)"
              unelevated
              color="warning"
              text-color="dark"
              size="sm"
              icon="build"
              label="Fix"
              :loading="fixing === h.id"
              @click="openFix(h)"
            />
            <q-btn
              flat
              dense
              size="sm"
              :color="h.stretched ? 'deep-purple' : 'grey-8'"
              :icon="h.stretched ? 'vpn_lock' : 'vpn_key_off'"
              :label="h.stretched ? 'Stretched' : 'Stretch'"
              :loading="toggling === h.id"
              @click="onToggleStretched(h)"
            >
              <q-tooltip>
                Cross a network boundary via VPN IP. LAN stays on file; probe uses stretched host when on.
              </q-tooltip>
            </q-btn>
            <q-btn flat dense size="sm" color="grey-8" icon="edit" label="Edit" @click="openEdit(h)" />
            <q-btn
              flat
              dense
              size="sm"
              color="negative"
              icon="delete"
              label="Remove"
              :disable="!!h.seed"
              @click="onDelete(h)"
            />
          </div>
        </div>
      </article>
    </div>

    <q-dialog v-model="formOpen" persistent>
      <q-card style="min-width: 420px">
        <q-card-section>
          <div class="text-h6">{{ editing ? 'Edit host' : 'Add MACHINE-HOST' }}</div>
        </q-card-section>
        <q-card-section class="q-pt-none">
          <q-input v-model="form.name" outlined dense label="Name" class="q-mb-sm" />
          <q-input v-model="form.sshUser" outlined dense label="SSH user" class="q-mb-sm" />
          <q-input
            v-model="form.sshHost"
            outlined
            dense
            label="SSH host / IP (LAN)"
            class="q-mb-sm"
            hint="e.g. 192.168.1.142 — used when Stretched is off"
          />
          <q-toggle
            v-model="form.stretched"
            color="deep-purple"
            label="Stretched (VPN reachability)"
            class="q-mb-xs"
          />
          <div class="text-caption text-grey-6 q-mb-sm">
            When the cluster cannot reach LAN — probe via WireGuard / VPN address instead.
          </div>
          <q-input
            v-model="form.stretchedHost"
            outlined
            dense
            label="Stretched host / VPN IP"
            class="q-mb-sm"
            hint="e.g. 10.50.0.3"
          />
          <q-input v-model.number="form.sshPort" type="number" outlined dense label="SSH port" class="q-mb-sm" />
          <q-input
            v-model="form.identityFile"
            outlined
            dense
            label="Identity file (private key path)"
            class="q-mb-sm"
            hint="~/.ssh/id_ecdsa or INVENTORY_SSH_KEY"
          />
          <q-input v-model="form.notes" outlined dense type="textarea" autogrow label="Notes" />
        </q-card-section>
        <q-card-actions align="right">
          <q-btn flat label="Cancel" v-close-popup />
          <q-btn color="primary" :label="editing ? 'Save' : 'Add'" :loading="saving" @click="onSaveForm" />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <q-dialog v-model="fixOpen" persistent>
      <q-card style="min-width: 480px; max-width: 560px">
        <q-card-section>
          <div class="text-h6">Fix host readiness</div>
          <div class="text-caption text-grey-7" v-if="fixHost">
            {{ fixHost.name }} · {{ fixHost.sshUser }}@{{ fixHost.sshHost }}
          </div>
        </q-card-section>
        <q-card-section class="q-pt-none">
          <div class="text-subtitle2 q-mb-sm">Actions</div>
          <q-option-group v-model="fixActions" :options="fixActionOptions" type="checkbox" color="warning" />
          <q-input
            v-model="sudoPassword"
            class="q-mt-md"
            outlined
            dense
            type="password"
            label="Sudo password (optional)"
            hint="Leave blank for passwordless sudo (-n). Never stored."
            autocomplete="new-password"
          />
          <q-banner v-if="fixLog.length" class="q-mt-md bg-grey-2" dense rounded>
            <pre class="fix-log">{{ fixLog.join('\n') }}</pre>
          </q-banner>
        </q-card-section>
        <q-card-actions align="right">
          <q-btn flat label="Close" v-close-popup />
          <q-btn
            color="warning"
            text-color="dark"
            icon="build"
            label="Run fix"
            :loading="fixing === fixHost?.id"
            :disable="!fixActions.length"
            @click="onFix"
          />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { Dialog, Notify } from 'quasar'
import {
  listInventory, createInventory, updateInventory, deleteInventory, probeInventory, fixInventory,
} from 'src/services/api'

const loading = ref(true)
const saving = ref(false)
const probing = ref(null)
const fixing = ref(null)
const toggling = ref(null)
const hosts = ref([])
const formOpen = ref(false)
const editing = ref(null)
const form = ref(emptyForm())

const fixOpen = ref(false)
const fixHost = ref(null)
const fixActions = ref([])
const sudoPassword = ref('')
const fixLog = ref([])

const ACTION_META = {
  'install-libvirt': { label: 'Install libvirt + qemu-kvm + start libvirtd' },
  'start-libvirtd': { label: 'Enable & start libvirtd' },
  'install-podman': { label: 'Install podman (runs curated mock-me-ee)' },
  'ensure-mock-me-ee': { label: 'Pull curated mock-me-ee (openshift-install + oc in container)' },
}

const fixActionOptions = computed(() => {
  const fromHost = (fixHost.value?.issues || [])
    .filter((i) => i.fixable && i.fixAction)
    .map((i) => i.fixAction)
  const uniq = [...new Set(fromHost.length ? fromHost : ['install-libvirt', 'install-podman'])]
  return uniq.map((id) => ({
    value: id,
    label: ACTION_META[id]?.label || id,
  }))
})

function emptyForm() {
  return {
    name: '',
    sshUser: 'dasm',
    sshHost: '',
    sshPort: 22,
    identityFile: '~/.ssh/id_ecdsa',
    notes: '',
    stretched: false,
    stretchedHost: '',
  }
}

function effectiveHost(h) {
  if (h.stretched && h.stretchedHost) return h.stretchedHost
  return h.sshHost
}

function statusColor(h) {
  const s = typeof h === 'string' ? h : h?.status
  if (s === 'reachable') return 'positive'
  if (s === 'partial') return 'warning'
  if (s === 'unreachable') return 'negative'
  return 'grey'
}

function statusLabel(h) {
  const s = typeof h === 'string' ? h : h?.status
  if (s === 'reachable') return 'libvirt ready'
  return s || 'unknown'
}

function statusIcon(h) {
  const s = typeof h === 'string' ? h : h?.status
  if (s === 'reachable') return isDeployBlocked(h) ? 'warning' : 'check_circle'
  if (s === 'partial') return 'warning'
  if (s === 'unreachable') return 'error'
  return 'dns'
}

function isDeployBlocked(h) {
  if (!h || typeof h !== 'object' || h.status !== 'reachable') return false
  const ee = String(h.facts?.mockMeEE || '').trim()
  if (ee === 'ready') return false
  const oi = String(h.facts?.openshiftInstall || '').trim()
  if (oi && oi !== 'missing' && oi !== 'ee') return false
  if (oi === 'ee') return false
  return true
}

function hasFixable(h) {
  return (h.issues || []).some((i) => i.fixable)
}

function issueIcon(h) {
  const errs = (h.issues || []).some((i) => i.severity === 'error' || i.id === 'openshift-install-missing')
  return errs || isDeployBlocked(h) ? 'error_outline' : 'warning'
}

function issueColor(h) {
  return issueIcon(h) === 'error_outline' ? 'orange-9' : 'warning'
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
    stretched: !!h.stretched,
    stretchedHost: h.stretchedHost || '',
  }
  formOpen.value = true
}

async function onToggleStretched(h) {
  if (!h.stretched && !h.stretchedHost) {
    Notify.create({
      type: 'warning',
      message: 'Set a stretched / VPN IP in Edit first (e.g. 10.50.0.3).',
    })
    openEdit(h)
    form.value.stretched = true
    return
  }
  toggling.value = h.id
  try {
    await updateInventory(h.id, { ...h, stretched: !h.stretched })
    Notify.create({
      type: 'positive',
      message: !h.stretched
        ? `Stretched on — probe via ${h.stretchedHost}`
        : `Stretched off — probe via LAN ${h.sshHost}`,
    })
    await load()
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    toggling.value = null
  }
}

function openFix(h) {
  fixHost.value = h
  const acts = [...new Set((h.issues || []).filter((i) => i.fixable && i.fixAction).map((i) => i.fixAction))]
  fixActions.value = acts.length ? acts : ['install-libvirt']
  sudoPassword.value = ''
  fixLog.value = []
  fixOpen.value = true
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
    const type = res.libvirtReady ? 'positive' : (res.authOK ? 'warning' : 'negative')
    Notify.create({ type, message: res.message, timeout: 7000 })
    await load()
    const updated = hosts.value.find((x) => x.id === h.id)
    if (updated && hasFixable(updated) && !res.libvirtReady && res.authOK) {
      openFix(updated)
    }
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    probing.value = null
  }
}

async function onFix() {
  if (!fixHost.value) return
  fixing.value = fixHost.value.id
  fixLog.value = ['Running remote fix over SSH…']
  try {
    const res = await fixInventory(fixHost.value.id, {
      actions: fixActions.value,
      sudoPassword: sudoPassword.value || undefined,
    })
    fixLog.value = res.log?.length ? res.log : [res.message]
    Notify.create({
      type: res.ok ? 'positive' : 'warning',
      message: res.message,
      timeout: 8000,
    })
    sudoPassword.value = ''
    await load()
    const updated = hosts.value.find((x) => x.id === fixHost.value.id)
    if (updated) fixHost.value = updated
    if (res.ok && res.probe?.libvirtReady) {
      fixOpen.value = false
    }
  } catch (e) {
    const msg = e.response?.data || e.message
    fixLog.value = [...fixLog.value, String(msg)]
    Notify.create({ type: 'negative', message: msg })
  } finally {
    fixing.value = null
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
.inventory-page {
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
  max-width: 1100px;
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

.title-row {
  display: flex;
  align-items: center;
  gap: 0.15rem;
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
  max-width: 36rem;
  color: #455a64;
  font-size: 0.95rem;
  line-height: 1.45;
}

.help-menu,
.detail-menu {
  padding: 0.85rem 1rem;
  font-size: 0.82rem;
  line-height: 1.45;
  color: #37474f;
}

.help-title {
  font-weight: 800;
  color: #0d2137;
  margin-bottom: 0.45rem;
}

.help-menu p {
  margin: 0 0 0.55rem;
}

.help-menu p:last-child {
  margin-bottom: 0;
}

.empty-state {
  text-align: center;
  padding: 3.5rem 1rem;
  max-width: 1100px;
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

.host-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 1.1rem;
  max-width: 1100px;
  margin: 0 auto;
}

.host-card {
  position: relative;
  display: flex;
  background: #fff;
  border-radius: 14px;
  border: 1px solid #b0bec5;
  box-shadow: 0 2px 0 rgba(13, 33, 55, 0.04), 0 10px 28px rgba(13, 33, 55, 0.08);
  overflow: hidden;
  transition: box-shadow 0.18s ease, transform 0.18s ease;
}

.host-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 0 rgba(13, 33, 55, 0.05), 0 16px 36px rgba(13, 33, 55, 0.12);
}

.host-card.deploy-blocked {
  border-color: #ffb74d;
  background: linear-gradient(180deg, #fff8f1 0%, #fff 45%);
}

.host-card.status-unreachable {
  border-color: #e57373;
}

.host-card.status-partial {
  border-color: #ffb74d;
}

.card-accent {
  width: 6px;
  flex-shrink: 0;
  background: #607d8b;
}

.status-reachable .card-accent { background: #2e7d32; }
.status-partial .card-accent { background: #ef6c00; }
.status-unreachable .card-accent { background: #c62828; }
.deploy-blocked .card-accent { background: #ef6c00; }

.card-body {
  flex: 1;
  min-width: 0;
  padding: 1rem 1.05rem 0.9rem;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.card-top {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
}

.card-identity {
  flex: 1;
  min-width: 0;
}

.name-row {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
}

.card-name {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 800;
  color: #0d2137;
  letter-spacing: -0.01em;
  line-height: 1.25;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chip-row {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
  margin-top: 0.35rem;
}

.icon-tools {
  display: flex;
  align-items: flex-start;
  gap: 0;
  flex-shrink: 0;
}

.card-endpoint {
  font-size: 0.82rem;
  color: #546e7a;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.card-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.3rem;
  margin-top: 0.35rem;
  padding-top: 0.75rem;
  border-top: 1px solid #cfd8dc;
}

.fact-line,
.issue-line {
  margin: 0.25rem 0;
  word-break: break-word;
}

.issue-line.error {
  color: #e65100;
  font-weight: 600;
}

.issue-line.warn {
  color: #e65100;
}

.fact-line.status-msg {
  color: #455a64;
  margin-bottom: 0.35rem;
}

.facts-grid {
  max-height: 240px;
  overflow: auto;
}

.fix-log {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 11px;
  max-height: 240px;
  overflow: auto;
}

@media (max-width: 600px) {
  .page-title { font-size: 1.65rem; }
  .host-grid { grid-template-columns: 1fr; }
}
</style>
