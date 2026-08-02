<template>
  <q-page padding class="wide">
    <div class="row items-center q-mb-md">
      <div class="col">
        <div class="text-h5">Inventory</div>
        <div class="text-caption text-grey-6">
          MACHINE-HOST targets — Probe (red / yellow / green) then Fix this when packages or services are missing.
        </div>
      </div>
      <q-btn color="primary" icon="add" label="Add host" @click="openCreate" />
    </div>

    <div class="row q-gutter-sm q-mb-md text-caption items-center">
      <q-badge color="negative">unreachable</q-badge>
      <span class="text-grey-7">no TCP / SSH</span>
      <q-badge color="warning">partial</q-badge>
      <span class="text-grey-7">SSH OK · missing libvirt</span>
      <q-badge color="positive">libvirt ready</q-badge>
      <span class="text-grey-7">can orchestrate infra</span>
      <q-badge color="orange-9">deploy blocked</q-badge>
      <span class="text-grey-7">need podman + curated mock-me-ee (openshift-install in container)</span>
    </div>

    <div v-if="loading" class="row justify-center q-my-xl"><q-spinner size="3em" color="primary" /></div>

    <q-list v-else bordered class="rounded-borders bg-white">
      <q-item v-for="h in hosts" :key="h.id" class="q-py-md" :class="{ 'deploy-blocked-row': isDeployBlocked(h) }">
        <q-item-section avatar>
          <q-avatar :color="statusColor(h)" text-color="white" :icon="statusIcon(h)" />
        </q-item-section>
        <q-item-section>
          <q-item-label class="text-weight-medium">
            {{ h.name }}
            <q-badge v-if="h.seed" color="primary" outline class="q-ml-sm">SEED</q-badge>
            <q-badge :color="statusColor(h)" class="q-ml-sm">{{ statusLabel(h) }}</q-badge>
            <q-badge v-if="isDeployBlocked(h)" color="orange-9" class="q-ml-sm">deploy blocked</q-badge>
            <q-badge v-else-if="h.status === 'reachable'" color="teal-8" class="q-ml-sm">deploy EE ok</q-badge>
          </q-item-label>
          <q-item-label caption>
            {{ h.sshUser }}@{{ effectiveHost(h) }}:{{ h.sshPort || 22 }}
            <q-badge v-if="h.stretched" color="deep-purple" outline dense class="q-ml-xs">stretched</q-badge>
            <span v-if="h.identityFile"> · key {{ h.identityFile }}</span>
          </q-item-label>
          <q-item-label caption v-if="h.stretched && h.sshHost" class="text-grey-6">
            LAN {{ h.sshHost }} · via VPN {{ h.stretchedHost }}
          </q-item-label>
          <q-item-label caption v-if="h.statusMessage" class="q-mt-xs">{{ h.statusMessage }}</q-item-label>
          <q-banner
            v-if="isDeployBlocked(h)"
            dense rounded
            class="q-mt-sm bg-orange-1 text-orange-10"
          >
            <template #avatar><q-icon name="block" color="orange-9" /></template>
            OCP Deploy needs the curated <code>mock-me-ee</code> image (openshift-install + oc inside the container).
            Host only needs podman — use <b>Fix this</b> → ensure-mock-me-ee, then Probe again.
          </q-banner>
          <div v-if="h.issues?.length" class="q-mt-sm">
            <div
              v-for="iss in h.issues"
              :key="iss.id"
              class="text-caption q-mb-xs"
              :class="iss.id === 'openshift-install-missing' || iss.severity === 'error' ? 'text-orange-10 text-weight-medium' : 'text-grey-8'"
            >
              · {{ iss.message }}
              <q-badge v-if="iss.fixable" dense color="warning" outline class="q-ml-xs">fixable</q-badge>
            </div>
          </div>
          <div v-if="h.facts && Object.keys(h.facts).length" class="q-mt-sm text-caption text-grey-7">
            <span v-for="(v, k) in h.facts" :key="k" class="q-mr-md" :class="{ 'text-orange-10 text-weight-bold': k === 'openshiftInstall' && v === 'missing' }">
              <b>{{ k }}</b>: {{ v }}
            </span>
          </div>
        </q-item-section>
        <q-item-section side>
          <div class="column q-gutter-sm">
            <q-btn
              unelevated color="primary" icon="sensors" label="Probe"
              :loading="probing === h.id"
              @click="onProbe(h)"
            />
            <q-btn
              v-if="hasFixable(h)"
              unelevated color="warning" text-color="dark" icon="build" label="Fix this"
              :loading="fixing === h.id"
              @click="openFix(h)"
            />
            <q-btn
              flat dense
              :color="h.stretched ? 'deep-purple' : 'grey-8'"
              :icon="h.stretched ? 'vpn_lock' : 'vpn_key_off'"
              :label="h.stretched ? 'Stretched on' : 'Stretched'"
              :loading="toggling === h.id"
              @click="onToggleStretched(h)"
            >
              <q-tooltip>
                Cross a network boundary via VPN IP (e.g. WireGuard). LAN stays on file; probe/fix use stretched host when on.
              </q-tooltip>
            </q-btn>
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
      Assume the host is subscribed and your SSH user can sudo (password optional in Fix).
      Fix installs <code>libvirt</code> / <code>podman</code> on the host, and pulls curated
      <code>mock-me-ee</code> (openshift-install + oc in the container — not on the host PATH).
      As-a-service probes need reachability + mounted identity (<code>INVENTORY_SSH_KEY</code>).
      Use <b>Stretched</b> when the cluster sits behind a boundary and must hit the host’s VPN address
      (WireGuard install on the MACHINE-HOST is a separate stretch step — not automated here yet).
    </q-banner>

    <q-dialog v-model="formOpen" persistent>
      <q-card style="min-width: 420px">
        <q-card-section>
          <div class="text-h6">{{ editing ? 'Edit host' : 'Add MACHINE-HOST' }}</div>
        </q-card-section>
        <q-card-section class="q-pt-none">
          <q-input v-model="form.name" outlined dense label="Name" class="q-mb-sm" />
          <q-input v-model="form.sshUser" outlined dense label="SSH user" class="q-mb-sm" />
          <q-input v-model="form.sshHost" outlined dense label="SSH host / IP (LAN)" class="q-mb-sm"
            hint="e.g. 192.168.1.142 — used when Stretched is off" />
          <q-toggle v-model="form.stretched" color="deep-purple" label="Stretched (VPN reachability)"
            class="q-mb-xs" />
          <div class="text-caption text-grey-6 q-mb-sm">
            When the cluster cannot reach LAN — probe via WireGuard / VPN address instead.
          </div>
          <q-input
            v-model="form.stretchedHost"
            outlined dense
            label="Stretched host / VPN IP"
            class="q-mb-sm"
            hint="e.g. 10.50.0.3 — optional until you need the boundary-crossing path"
          />
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
  // Legacy: host PATH installer still unblocks Deploy
  if (oi && oi !== 'missing' && oi !== 'ee') return false
  if (oi === 'ee') return false
  return true
}

function hasFixable(h) {
  return (h.issues || []).some((i) => i.fixable)
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
.wide { max-width: 1100px; margin: 0 auto; }
.deploy-blocked-row {
  background: #fff8f1;
}
.fix-log {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 11px;
  max-height: 240px;
  overflow: auto;
}
</style>
