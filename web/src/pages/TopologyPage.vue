<template>
  <q-page padding class="wide topo-page">
    <div class="row items-center q-mb-sm">
      <q-btn flat dense icon="arrow_back" :to="{ name: 'mockups' }" />
      <div class="col">
        <div class="text-h5">{{ mockup?.metadata?.name || 'Topology' }}</div>
        <div class="text-caption text-grey-6" v-if="mockup">
          {{ objectSummary }}
        </div>
      </div>

      <q-btn-dropdown outline color="primary" icon="add" label="Add" class="q-mr-sm">
        <q-list style="min-width: 280px">
          <q-item clickable v-close-popup @click="onAddCluster">
            <q-item-section avatar><q-icon name="developer_board" color="primary" /></q-item-section>
            <q-item-section>
              <q-item-label>Deployment cluster (v)Host</q-item-label>
              <q-item-label caption>Managed by ACM · adds compact guest set</q-item-label>
            </q-item-section>
          </q-item>
          <q-separator />
          <q-item dense>
            <q-item-section>
              <q-item-label caption>
                Machine host, VyOS gateway, and mgmt cluster are singletons — select them on the canvas to edit.
              </q-item-label>
            </q-item-section>
          </q-item>
        </q-list>
      </q-btn-dropdown>
      <q-btn color="primary" icon="save" label="Save" :loading="saving" @click="persist" />
    </div>

    <div v-if="loading" class="row justify-center q-my-xl"><q-spinner size="3em" color="primary" /></div>
    <template v-else-if="mockup">
      <div class="legend q-mb-md">
        <span class="legend-item"><i class="swatch swatch-host" /> Machine host</span>
        <span class="legend-item"><i class="swatch swatch-gw" /> Gateway</span>
        <span class="legend-item"><i class="swatch swatch-mgmt" /> Mgmt (runs ACM)</span>
        <span class="legend-item"><i class="swatch swatch-dep" /> Deployment (managed)</span>
        <span class="legend-hint">Drag to arrange · click to inspect · Edit for full form</span>
      </div>

      <div class="row q-col-gutter-lg">
        <div class="col-12 col-lg-8">
          <TopologyCanvas
            :mockup="mockup"
            :selected-id="selected?.id"
            @select="onSelect"
            @move="onMove"
          />
        </div>

        <div class="col-12 col-lg-4">
          <aside class="inspector">
            <template v-if="!selected">
              <div class="inspector-empty">
                <div class="inspector-empty-title">Select an object</div>
                <div class="inspector-empty-body">
                  Click a node on the canvas to see its class, role, and ACM posture.
                </div>
                <div class="inspector-actions q-mt-md">
                  <q-btn flat color="primary" label="Wizard" icon="playlist_play"
                    :to="{ name: 'wizard', params: { id } }" />
                  <q-btn flat color="primary" label="Derive" icon="description"
                    :loading="deriving" @click="onDerive" />
                </div>
              </div>
            </template>

            <template v-else>
              <div class="inspector-head">
                <div>
                  <div class="inspector-class">{{ selectedMeta.classLabel }}</div>
                  <div class="inspector-title">{{ selectedMeta.title }}</div>
                </div>
                <q-btn flat dense round icon="close" @click="selected = null" />
              </div>

              <div class="inspector-role">{{ selectedMeta.roleLine }}</div>
              <div class="inspector-acm">{{ selectedMeta.acmLine }}</div>

              <dl class="inspector-facts">
                <div v-for="f in selectedMeta.facts" :key="f.k" class="fact">
                  <dt>{{ f.k }}</dt>
                  <dd>{{ f.v }}</dd>
                </div>
              </dl>

              <div class="inspector-actions">
                <q-btn color="primary" unelevated label="Edit details" icon="edit" @click="openEditor" />
                <q-btn
                  v-if="selected.kind === 'cluster' && mockup.spec.clusters.length > 1"
                  flat color="negative" label="Remove" icon="delete"
                  @click="onDeleteCluster(selectedNodeData)"
                />
              </div>
            </template>
          </aside>

          <div class="object-rail q-mt-md">
            <div class="object-rail-head">
              <span>Objects</span>
              <q-btn flat dense size="sm" icon="add" color="primary" @click="onAddCluster">
                <q-tooltip>Add deployment cluster</q-tooltip>
              </q-btn>
            </div>
            <button
              v-for="row in objectRows"
              :key="row.id"
              type="button"
              class="object-row"
              :class="{ active: selected?.id === row.id }"
              @click="selectById(row.id, row.kind)"
            >
              <span class="object-row-dot" :class="'dot-' + row.kind" />
              <span class="object-row-main">
                <span class="object-row-title">{{ row.title }}</span>
                <span class="object-row-sub">{{ row.sub }}</span>
              </span>
            </button>
          </div>

          <details class="net-details q-mt-md">
            <summary>Lab network</summary>
            <div class="net-fields q-pt-sm">
              <q-input v-model="mockup.spec.baseDomain" outlined dense label="Base domain" class="q-mb-sm" />
              <q-input v-model="mockup.spec.network.machineCIDR" outlined dense label="LAN CIDR" class="q-mb-sm" />
              <q-input v-model="mockup.spec.network.gateway" outlined dense label="Gateway IP" class="q-mb-sm" />
              <q-input v-model="mockup.spec.network.apiVIP" outlined dense label="Default API VIP" class="q-mb-sm" />
              <q-input v-model="mockup.spec.network.ingressVIP" outlined dense label="Default Ingress VIP" />
            </div>
          </details>
        </div>
      </div>
    </template>

    <NodeEditDialog
      v-model="editOpen"
      :kind="editKind"
      :node="editNode"
      @save="onSaveNode"
    />
  </q-page>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { Dialog, Notify } from 'quasar'
import TopologyCanvas from 'src/components/TopologyCanvas.vue'
import NodeEditDialog from 'src/components/NodeEditDialog.vue'
import {
  getMockup, saveMockup, patchLayout, addCluster, deleteCluster, deriveMockup, imageSetName,
} from 'src/services/api'

const props = defineProps({ id: { type: String, required: true } })

const mockup = ref(null)
const loading = ref(true)
const saving = ref(false)
const deriving = ref(false)
const selected = ref(null)
const editOpen = ref(false)
const editKind = ref('hub')
const editNode = ref(null)
let layoutTimer = null

const objectSummary = computed(() => {
  if (!mockup.value) return ''
  const n = mockup.value.spec.clusters?.length || 0
  return `1 machine host · 1 gateway · 1 mgmt · ${n} deployment`
})

const objectRows = computed(() => {
  const m = mockup.value
  if (!m?.spec) return []
  const rows = []
  const ih = m.spec.infraHost
  if (ih?.id) {
    rows.push({
      id: ih.id, kind: 'infraHost',
      title: ih.label || 'MACHINE-HOST',
      sub: 'Host machine · libvirtd',
    })
  }
  const gw = m.spec.gateway
  if (gw?.id) {
    rows.push({
      id: gw.id, kind: 'gateway',
      title: gw.label || 'VYOS-GW',
      sub: 'Edge · WAN/LAN',
    })
  }
  if (m.spec.hub?.id) {
    rows.push({
      id: m.spec.hub.id, kind: 'hub',
      title: m.spec.hub.label || 'MGMT-CLUSTER',
      sub: 'Mgmt (v)Host · runs ACM',
    })
  }
  if (m.spec.acm?.id) {
    rows.push({
      id: m.spec.acm.id, kind: 'acm',
      title: m.spec.acm.label || 'ACM',
      sub: m.spec.acm.enabled ? 'Governs deployments' : 'Disabled',
    })
  }
  for (const c of m.spec.clusters || []) {
    rows.push({
      id: c.id, kind: 'cluster',
      title: c.label || c.name,
      sub: `Deployment (v)Host · managed · ${c.phase || 'planned'}`,
    })
  }
  return rows
})

const selectedNodeData = computed(() => {
  if (!selected.value || !mockup.value) return null
  const k = selected.value.kind
  if (k === 'infraHost') return mockup.value.spec.infraHost
  if (k === 'gateway') return mockup.value.spec.gateway
  if (k === 'hub') return mockup.value.spec.hub
  if (k === 'acm') return mockup.value.spec.acm
  return mockup.value.spec.clusters.find((c) => c.id === selected.value.id) || null
})

const selectedMeta = computed(() => {
  const n = selected.value
  const d = selectedNodeData.value
  if (!n || !d) {
    return { classLabel: '', title: '', roleLine: '', acmLine: '', facts: [] }
  }
  if (n.kind === 'infraHost') {
    return {
      classLabel: 'Host machine',
      title: d.label || 'MACHINE-HOST',
      roleLine: 'Role: libvirt / podman host (not an OCP node)',
      acmLine: 'ACM: n/a — provides compute for guests',
      facts: [
        { k: 'SSH', v: d.sshHost || '—' },
        { k: 'OS', v: `${d.os || '—'} · ${d.kind || '—'}` },
        { k: 'Size', v: `${d.cpu}c / ${Math.round((d.memoryMiB || 0) / 1024)}G` },
        { k: 'Disks', v: (d.disks || []).map((x) => `${x.sizeGiB}G ${x.role || ''}`).join(' + ') || `${d.diskGiB}G` },
        { k: 'NICs', v: (d.nics || []).map((x) => x.role || x.mode).join(', ') || '—' },
      ],
    }
  }
  if (n.kind === 'gateway') {
    return {
      classLabel: 'Edge gateway',
      title: d.label || 'VYOS-GW',
      roleLine: 'Role: NAT / firewall between bridge WAN and lab LAN',
      acmLine: 'ACM: n/a — lab edge router',
      facts: [
        { k: 'LAN', v: `${d.lanCIDR || '—'} · ${d.lanIP || ''}` },
        { k: 'WAN', v: d.wanBridge || 'bridged' },
        { k: 'Phase', v: d.phase || 'planned' },
        { k: 'Size', v: `${d.cpu}c / ${d.memoryMiB}MiB` },
      ],
    }
  }
  if (n.kind === 'hub') {
    return {
      classLabel: 'Cluster (v)Host',
      title: d.label || 'MGMT-CLUSTER',
      roleLine: 'Role: Management cluster guest',
      acmLine: 'ACM: runs ACM (governance hub)',
      facts: [
        { k: 'Hostname', v: d.hostname || '—' },
        { k: 'Profile', v: `${d.profile || '—'} · OCP ${d.version || '—'}` },
        { k: 'Size', v: `${d.cpu}c / ${Math.round((d.memoryMiB || 0) / 1024)}G / ${d.diskGiB}G` },
        { k: 'IP', v: d.ip || '—' },
      ],
    }
  }
  if (n.kind === 'acm') {
    return {
      classLabel: 'Operator',
      title: d.label || 'ACM',
      roleLine: 'Role: MultiClusterHub on management cluster',
      acmLine: d.enabled ? 'ACM: enabled — manages deployment clusters' : 'ACM: disabled',
      facts: [
        { k: 'MCE', v: d.mceChannel || '—' },
        { k: 'ACM', v: d.acmChannel || '—' },
      ],
    }
  }
  // cluster
  return {
    classLabel: 'Cluster (v)Host',
    title: d.label || d.name,
    roleLine: 'Role: Deployment cluster guest set',
    acmLine: 'ACM: managed by hub ACM',
    facts: [
      { k: 'Name', v: d.name || '—' },
      { k: 'Phase', v: d.phase || 'planned' },
      { k: 'Nodes', v: `${d.count}× ${d.cpu}c / ${Math.round((d.memoryMiB || 0) / 1024)}G` },
      { k: 'ImageSet', v: d.clusterImageSet || '—' },
      { k: 'API VIP', v: d.apiVIP || '—' },
    ],
  }
})

async function load() {
  loading.value = true
  try {
    mockup.value = await getMockup(props.id)
  } catch (e) {
    Notify.create({ type: 'negative', message: e.message })
  } finally {
    loading.value = false
  }
}

function onSelect(n) {
  selected.value = { id: n.id, kind: n.kind }
}

function selectById(id, kind) {
  selected.value = { id, kind }
}

function openEditor() {
  const d = selectedNodeData.value
  if (!d || !selected.value) return
  editKind.value = selected.value.kind
  editNode.value = { ...d }
  editOpen.value = true
}

function onMove({ id, x, y }) {
  if (!mockup.value.layout.nodes) mockup.value.layout.nodes = {}
  mockup.value.layout.nodes[id] = { x, y }
  clearTimeout(layoutTimer)
  layoutTimer = setTimeout(async () => {
    try {
      await patchLayout(props.id, mockup.value.layout)
    } catch { /* ignore */ }
  }, 400)
}

async function onSaveNode({ kind, node }) {
  if (kind === 'infraHost') mockup.value.spec.infraHost = { ...mockup.value.spec.infraHost, ...node }
  else if (kind === 'gateway') {
    mockup.value.spec.gateway = { ...mockup.value.spec.gateway, ...node }
    if (node.lanCIDR) mockup.value.spec.network.machineCIDR = node.lanCIDR
    if (node.lanIP) {
      mockup.value.spec.network.gateway = node.lanIP
      mockup.value.spec.network.dns = node.lanIP
    }
    if (node.lanNetwork) mockup.value.spec.infraHost.networkName = node.lanNetwork
  }
  else if (kind === 'hub') mockup.value.spec.hub = { ...mockup.value.spec.hub, ...node }
  else if (kind === 'acm') mockup.value.spec.acm = { ...mockup.value.spec.acm, ...node }
  else {
    const i = mockup.value.spec.clusters.findIndex((c) => c.id === node.id)
    if (i >= 0) {
      if (node.version && (!node.clusterImageSet || node.clusterImageSet.startsWith('img'))) {
        node.clusterImageSet = imageSetName(node.version)
      }
      mockup.value.spec.clusters[i] = { ...mockup.value.spec.clusters[i], ...node }
    }
  }
  await persist()
}

async function persist() {
  saving.value = true
  try {
    mockup.value = await saveMockup(props.id, mockup.value)
    Notify.create({ type: 'positive', message: 'Saved.' })
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    saving.value = false
  }
}

async function onAddCluster() {
  try {
    const res = await addCluster(props.id)
    mockup.value = res.mockup
    selected.value = { id: res.cluster.id, kind: 'cluster' }
    Notify.create({ type: 'positive', message: `Added ${res.cluster.label}` })
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  }
}

function onDeleteCluster(c) {
  if (!c) return
  Dialog.create({
    title: 'Remove deployment cluster?',
    message: `${c.label || c.name} will be removed from this MockUp.`,
    cancel: true,
    persistent: true,
  }).onOk(async () => {
    try {
      mockup.value = await deleteCluster(props.id, c.id)
      if (selected.value?.id === c.id) selected.value = null
      Notify.create({ type: 'positive', message: 'Removed.' })
    } catch (e) {
      Notify.create({ type: 'negative', message: e.response?.data || e.message })
    }
  })
}

async function onDerive() {
  deriving.value = true
  try {
    await persist()
    const res = await deriveMockup(props.id)
    Notify.create({ type: 'positive', message: `YAML at ${Object.values(res.paths).join(', ')}`, timeout: 6000 })
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    deriving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem 1.25rem;
  align-items: center;
  font-size: 0.8rem;
  color: #546e7a;
}
.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}
.legend-hint {
  margin-left: auto;
  color: #90a4ae;
  font-size: 0.75rem;
}
.swatch {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 2px;
}
.swatch-host { background: #37474f; }
.swatch-gw { background: #e65100; }
.swatch-mgmt { background: #1a237e; }
.swatch-dep { background: #1565c0; }

.inspector {
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  padding: 1rem 1.1rem;
  background: #fff;
  min-height: 220px;
}
.inspector-empty-title {
  font-weight: 600;
  font-size: 1rem;
  color: #263238;
}
.inspector-empty-body {
  margin-top: 0.35rem;
  color: #78909c;
  font-size: 0.875rem;
  line-height: 1.4;
}
.inspector-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 0.5rem;
}
.inspector-class {
  font-size: 0.7rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: #90a4ae;
  font-weight: 600;
}
.inspector-title {
  font-size: 1.15rem;
  font-weight: 700;
  color: #0d47a1;
  line-height: 1.2;
}
.inspector-role,
.inspector-acm {
  margin-top: 0.55rem;
  font-size: 0.875rem;
  color: #455a64;
  line-height: 1.35;
}
.inspector-acm {
  color: #00838f;
  font-weight: 500;
}
.inspector-facts {
  margin: 1rem 0 0;
  padding: 0;
}
.fact {
  display: grid;
  grid-template-columns: 5.5rem 1fr;
  gap: 0.5rem;
  padding: 0.4rem 0;
  border-top: 1px solid #f0f0f0;
  font-size: 0.85rem;
}
.fact dt {
  margin: 0;
  color: #90a4ae;
  font-weight: 500;
}
.fact dd {
  margin: 0;
  color: #37474f;
  word-break: break-word;
}
.inspector-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-top: 1rem;
}

.object-rail {
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  overflow: hidden;
  background: #fff;
}
.object-rail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.55rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: #78909c;
  border-bottom: 1px solid #eee;
}
.object-row {
  display: flex;
  align-items: flex-start;
  gap: 0.65rem;
  width: 100%;
  text-align: left;
  border: 0;
  border-bottom: 1px solid #f5f5f5;
  background: transparent;
  padding: 0.65rem 0.75rem;
  cursor: pointer;
}
.object-row:last-child { border-bottom: 0; }
.object-row:hover { background: #f7fafc; }
.object-row.active { background: #e3f2fd; }
.object-row-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-top: 0.4rem;
  flex-shrink: 0;
}
.dot-infraHost { background: #37474f; }
.dot-gateway { background: #e65100; }
.dot-hub { background: #1a237e; }
.dot-acm { background: #00838f; }
.dot-cluster { background: #1565c0; }
.object-row-main { display: flex; flex-direction: column; min-width: 0; }
.object-row-title {
  font-size: 0.875rem;
  font-weight: 600;
  color: #263238;
}
.object-row-sub {
  font-size: 0.75rem;
  color: #90a4ae;
  margin-top: 0.1rem;
}

.net-details {
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  padding: 0.55rem 0.85rem 0.85rem;
  background: #fff;
  color: #546e7a;
  font-size: 0.85rem;
}
.net-details summary {
  cursor: pointer;
  font-weight: 600;
  color: #455a64;
  list-style: none;
}
.net-details summary::-webkit-details-marker { display: none; }
</style>
