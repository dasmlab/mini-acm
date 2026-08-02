<template>
  <q-page padding class="wide topo-page">
    <div class="row items-center q-mb-sm">
      <q-btn flat dense icon="arrow_back" :to="{ name: 'mockups' }" />
      <div class="col">
        <div class="row items-center no-wrap q-gutter-sm">
          <div class="text-h5">{{ mockup?.metadata?.name || 'Topology' }}</div>
          <q-badge v-if="mockup?.status?.phase" :color="phaseColor(mockup.status.phase)" class="text-capitalize">
            {{ mockup.status.phase }}
          </q-badge>
        </div>
        <div class="text-caption text-grey-6" v-if="mockup">
          {{ objectSummary }}
        </div>
      </div>

      <q-btn-toggle
        v-if="mockup"
        v-model="canvasMode"
        toggle-color="primary"
        unelevated
        dense
        class="q-mr-sm"
        :options="[
          { label: 'Guided', value: 'guided' },
          { label: 'Free-form', value: 'freeform' },
        ]"
        @update:model-value="onCanvasMode"
      />
      <q-btn
        v-if="showClean"
        outline
        color="warning"
        icon="cleaning_services"
        label="Clean"
        class="q-mr-sm"
        :loading="cleaning"
        @click="onClean"
      >
        <q-tooltip>Reset deploy state so Validate/Deploy can run again (leaves host VMs in place)</q-tooltip>
      </q-btn>
      <q-btn
        outline
        color="deep-purple-7"
        icon="rule"
        label="Validate"
        class="q-mr-sm"
        :loading="validating"
        :disable="isLocked"
        @click="onValidate"
      />
      <q-btn
        outline
        color="orange-9"
        icon="rocket_launch"
        label="Deploy"
        class="q-mr-sm"
        :loading="deploying"
        :disable="isLocked || !!eeBlockReason"
        @click="onDeploy"
      >
        <q-tooltip v-if="eeBlockReason">{{ eeBlockReason }}</q-tooltip>
      </q-btn>
      <q-btn-dropdown outline color="primary" icon="add" label="Add" class="q-mr-sm">
        <q-list style="min-width: 340px">
          <q-item-label header>{{ isFreeForm ? 'Free-form palette' : 'Guided add' }}</q-item-label>
          <q-item v-if="isACMMultiCluster" clickable v-close-popup @click="onAddCluster">
            <q-item-section avatar><q-icon name="developer_board" color="primary" /></q-item-section>
            <q-item-section>
              <q-item-label>OCP-DEPLOY cluster</q-item-label>
              <q-item-label caption>Managed OCP (+ derived vHosts)</q-item-label>
            </q-item-section>
          </q-item>
          <q-item v-else disable>
            <q-item-section avatar><q-icon name="developer_board" color="grey" /></q-item-section>
            <q-item-section>
              <q-item-label>OCP-DEPLOY (ACM Multi-Cluster only)</q-item-label>
              <q-item-label caption>Single SNO style stops at OCP-MGMT</q-item-label>
            </q-item-section>
          </q-item>
          <template v-if="isFreeForm">
            <q-item clickable v-close-popup @click="onAddOrphanVHost">
              <q-item-section avatar><q-icon name="dns" color="blue-grey" /></q-item-section>
              <q-item-section>
                <q-item-label>vHost (orphan)</q-item-label>
                <q-item-label caption>Teaching drop — needs a payload on Validate</q-item-label>
              </q-item-section>
            </q-item>
            <q-item clickable v-close-popup @click="onAddAppliance('haproxy')">
              <q-item-section avatar><q-icon name="router" color="brown" /></q-item-section>
              <q-item-section>
                <q-item-label>HAProxy appliance</q-item-label>
                <q-item-label caption>Link to a vHost (runsOn) — stub payload</q-item-label>
              </q-item-section>
            </q-item>
            <q-item clickable v-close-popup @click="onAddAppliance('other')">
              <q-item-section avatar><q-icon name="extension" color="brown" /></q-item-section>
              <q-item-section>
                <q-item-label>Other appliance</q-item-label>
                <q-item-label caption>Generic NF / middleware on a vHost</q-item-label>
              </q-item-section>
            </q-item>
            <q-item disable>
              <q-item-section avatar><q-icon name="device_hub" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label>Arbiter (2cp+worker)</q-item-label>
                <q-item-label caption>Later — tiny vHost + stack between control planes</q-item-label>
              </q-item-section>
            </q-item>
            <q-separator />
            <q-item-label header>Show / hide rack pieces</q-item-label>
            <q-item clickable v-close-popup @click="toggleOmit('omitHub')">
              <q-item-section>
                <q-item-label>{{ mockup.spec.canvas?.omitHub ? 'Show' : 'Hide' }} mgmt cluster</q-item-label>
              </q-item-section>
            </q-item>
            <q-item clickable v-close-popup @click="toggleOmit('omitACM')">
              <q-item-section>
                <q-item-label>{{ mockup.spec.canvas?.omitACM ? 'Show' : 'Hide' }} ACM</q-item-label>
              </q-item-section>
            </q-item>
            <q-item clickable v-close-popup @click="toggleOmit('omitGateway')">
              <q-item-section>
                <q-item-label>{{ mockup.spec.canvas?.omitGateway ? 'Show' : 'Hide' }} VyOS gateway</q-item-label>
              </q-item-section>
            </q-item>
            <q-item clickable v-close-popup @click="toggleOmit('omitHost')">
              <q-item-section>
                <q-item-label>{{ mockup.spec.canvas?.omitHost ? 'Show' : 'Hide' }} MACHINE-HOST</q-item-label>
              </q-item-section>
            </q-item>
            <q-item clickable v-close-popup @click="stripForTeaching">
              <q-item-section>
                <q-item-label>Strip to blank teaching canvas</q-item-label>
                <q-item-label caption>Hide rack + clear deployments / orphans</q-item-label>
              </q-item-section>
            </q-item>
            <q-separator />
            <q-item clickable v-close-popup @click="toggleShowRelations">
              <q-item-section>
                <q-item-label>{{ mockup.spec.canvas?.showRelations ? 'Hide' : 'Show' }} relation edges</q-item-label>
                <q-item-label caption>Free-form defaults to no constrained lines</q-item-label>
              </q-item-section>
            </q-item>
            <q-item disable>
              <q-item-section>
                <q-item-label>Promote → guided MockUp</q-item-label>
                <q-item-label caption>TODO later — not supported; rebuild in Guided</q-item-label>
              </q-item-section>
            </q-item>
          </template>
          <template v-else>
            <q-item disable>
              <q-item-section avatar><q-icon name="router" color="grey" /></q-item-section>
              <q-item-section>
                <q-item-label>HAProxy / VIP front</q-item-label>
                <q-item-label caption>Use Free-form to drop a stub, or later guided add</q-item-label>
              </q-item-section>
            </q-item>
            <q-separator />
            <q-item dense>
              <q-item-section>
                <q-item-label caption>
                  Guided: host / adapter / VyOS / mgmt are singletons — select to edit. Switch to Free-form to teach with orphan drops.
                </q-item-label>
              </q-item-section>
            </q-item>
          </template>
        </q-list>
      </q-btn-dropdown>
      <q-btn color="primary" icon="save" label="Save" :loading="saving" @click="persist" />
    </div>

    <div v-if="loading" class="row justify-center q-my-xl"><q-spinner size="3em" color="primary" /></div>
    <template v-else-if="mockup">
      <div class="layer-bar q-mb-sm">
        <button
          v-for="opt in layerOptions"
          :key="opt.id"
          type="button"
          class="layer-tab"
          :class="{ active: layer === opt.id }"
          @click="layer = opt.id"
        >
          <span class="layer-tab-title">{{ opt.title }}</span>
          <span class="layer-tab-sub">{{ opt.sub }}</span>
        </button>
      </div>

      <div class="legend q-mb-md">
        <span v-if="layer === 'all' || layer === 'infra'" class="legend-item"><i class="swatch swatch-host" /> Host / adapter</span>
        <span v-if="layer === 'all' || layer === 'infra'" class="legend-item"><i class="swatch swatch-vhost" /> vHost</span>
        <span v-if="layer === 'all' || layer === 'infra'" class="legend-item"><i class="swatch swatch-gw" /> VyOS (RTR)</span>
        <span v-if="layer === 'network'" class="legend-item"><i class="swatch swatch-phy" /> PRI-PHY-NIC</span>
        <span v-if="layer === 'network'" class="legend-item"><i class="swatch swatch-vswitch" /> vSwitch</span>
        <span v-if="layer === 'network'" class="legend-item"><i class="swatch swatch-vnic" /> vNIC</span>
        <span v-if="layer === 'all' || layer === 'cluster'" class="legend-item"><i class="swatch swatch-mgmt" /> OCP-MGMT</span>
        <span v-if="(layer === 'all' || layer === 'cluster') && isACMMultiCluster" class="legend-item"><i class="swatch swatch-dep" /> OCP-DEPLOY</span>
        <span v-if="(layer === 'all' || layer === 'cluster' || layer === 'app') && isACMMultiCluster" class="legend-item"><i class="swatch swatch-acm" /> ACM</span>
        <span class="legend-hint">{{ layerHint }}</span>
      </div>

      <div class="row q-col-gutter-lg">
        <div class="col-12 col-lg-8">
          <TopologyCanvas
            :mockup="mockup"
            :selected-id="selected?.id"
            :layer="layer"
            :free-form="isFreeForm"
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
                    :disable="isLocked"
                    :to="{ name: 'wizard', params: { id } }" />
                  <q-btn flat color="primary" label="Derive" icon="description"
                    :disable="isLocked" :loading="deriving" @click="onDerive" />
                  <q-btn flat color="deep-purple-7" label="Validate" icon="rule"
                    :disable="isLocked" :loading="validating" @click="onValidate" />
                  <q-btn flat color="orange-9" label="Deploy" icon="rocket_launch"
                    :disable="isLocked || !!eeBlockReason" :loading="deploying" @click="onDeploy">
                    <q-tooltip v-if="eeBlockReason">{{ eeBlockReason }}</q-tooltip>
                  </q-btn>
                  <q-btn v-if="showClean" flat color="warning" label="Clean" icon="cleaning_services"
                    :loading="cleaning" @click="onClean" />
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
                <q-btn
                  v-if="selectedMeta.editable !== false"
                  color="primary" unelevated
                  :label="selected.kind === 'vhost' ? 'Edit parent' : 'Edit details'"
                  icon="edit" @click="openEditor"
                />
                <q-btn
                  v-if="selected.kind === 'cluster' && (isFreeForm || mockup.spec.clusters.length > 1)"
                  flat color="negative" label="Remove" icon="delete"
                  @click="onDeleteCluster(selectedNodeData)"
                />
                <q-btn
                  v-if="selected.kind === 'vhost' && selectedMeta.orphan"
                  flat color="negative" label="Remove vHost" icon="delete"
                  @click="onRemoveOrphan(selected.id)"
                />
                <q-btn
                  v-if="selected.kind === 'appliance'"
                  flat color="negative" label="Remove" icon="delete"
                  @click="onRemoveOrphan(selected.id)"
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

    <DeployAssemblyDialog
      v-model="deployOpen"
      :mockup-id="id"
      :mockup-name="mockup?.metadata?.name || ''"
      :initial-job="deployJob"
      @finished="onDeployFinished"
      @cleaned="load"
    />

    <ValidateWalkDialog
      v-model="validateOpen"
      :title="mockup?.metadata?.name || 'Topology'"
      :result="validateResult"
    />
  </q-page>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { Dialog, Notify } from 'quasar'
import TopologyCanvas from 'src/components/TopologyCanvas.vue'
import NodeEditDialog from 'src/components/NodeEditDialog.vue'
import DeployAssemblyDialog from 'src/components/DeployAssemblyDialog.vue'
import ValidateWalkDialog from 'src/components/ValidateWalkDialog.vue'
import {
  getMockup, saveMockup, patchLayout, addCluster, deleteCluster, deriveMockup, validateMockup, deployMockup, cleanMockup, listInventory, imageSetName,
} from 'src/services/api'
import { enumerateVHosts, ensureCanvas, newOrphanId } from 'src/lib/vhosts'
import { enumerateNetwork } from 'src/lib/network'

const props = defineProps({ id: { type: String, required: true } })

const mockup = ref(null)
const inventory = ref([])
const loading = ref(true)
const saving = ref(false)
const deriving = ref(false)
const validating = ref(false)
const deploying = ref(false)
const cleaning = ref(false)
const deployOpen = ref(false)
const deployJob = ref(null)
const validateOpen = ref(false)
const validateResult = ref(null)
const selected = ref(null)
const editOpen = ref(false)
const editKind = ref('hub')
const editNode = ref(null)
const layer = ref('all')
const canvasMode = ref('guided')
let layoutTimer = null

const isFreeForm = computed(() => canvasMode.value === 'freeform')
const isSingleSNO = computed(() => mockup.value?.spec?.style === 'single-sno-ocp')
const isACMMultiCluster = computed(() => !mockup.value?.spec?.style || mockup.value.spec.style === 'acm-multi-cluster')

function phaseColor(phase) {
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

const isLocked = computed(() => {
  const p = mockup.value?.status?.phase || ''
  return p === 'failed' || p === 'deploying'
})

const showClean = computed(() => {
  const p = mockup.value?.status?.phase || ''
  return p === 'failed' || p === 'deploying' || p === 'deployed'
})

const eeBlockReason = computed(() => {
  const list = inventory.value || []
  const h = list.find((x) => x.seed)
    || list.find((x) => x.status === 'reachable')
    || list[0]
    || null
  if (!h) return 'No Inventory MACHINE-HOST — add and Probe one first'
  if (h.status === 'unreachable') return `${h.name} is unreachable — Probe Inventory`
  if (h.status === 'partial') return `${h.name} is partial — Fix this on Inventory`
  const oi = String(h.facts?.openshiftInstall || '').trim()
  const ee = String(h.facts?.mockMeEE || '').trim()
  if (ee !== 'ready' && (!oi || oi === 'missing')) {
    return `curated mock-me-ee missing on ${h.name} — Probe → Fix this (ensure-mock-me-ee)`
  }
  const pod = String(h.facts?.podman || '').trim().toLowerCase()
  if (!pod || pod === 'missing' || pod.includes('missing')) {
    return `podman missing on ${h.name} — Fix this on Inventory`
  }
  return ''
})

const layerOptions = computed(() => {
  const all = [
    { id: 'all', title: 'Full rack', sub: 'High-level bands' },
    { id: 'infra', title: 'Infrastructure', sub: 'Host · adapter · vHosts' },
    { id: 'network', title: 'Network', sub: 'vSwitch · vNICs · PRI-PHY' },
    { id: 'cluster', title: isSingleSNO.value ? 'OCP-MGMT' : 'Cluster mgmt', sub: isSingleSNO.value ? 'SNO only' : 'ACM · home + managed' },
    { id: 'app', title: 'Application', sub: 'ACM payload' },
  ]
  if (isSingleSNO.value) {
    return all.filter((o) => o.id !== 'app')
  }
  return all
})

const layerHint = computed(() => {
  if (layer.value === 'infra') {
    return isSingleSNO.value
      ? 'Adapter → vHosts (GW + SNO) · VyOS on vHost-GW'
      : 'Adapter → vHosts (GW + MGMT SNO + 3× per deployment) · VyOS on vHost-GW'
  }
  if (layer.value === 'network') {
    return 'PRI-PHY-NIC ↔ VyOS eth0 WAN · eth1 LAN (.1) + guest vNICs ↔ vSwitch'
  }
  if (layer.value === 'cluster') {
    return isSingleSNO.value
      ? 'OCP-MGMT (SNO) — stop before ACM; promote style to ACM Multi-Cluster to add ACM + OCP-DEPLOY'
      : 'OCP objects — ACM on OCP-MGMT governs OCP-DEPLOY (not full self-mgmt yet)'
  }
  if (layer.value === 'app') {
    return 'ACM today · Ansible / GitOps payloads can land on clusters later'
  }
  return 'Use Network tab for vNICs · Infra for machines'
})

/** Object-list filters per view. Infra lists vHosts (not OCP cluster boxes). */
const LAYER_ROWS = {
  infra: new Set(['infraHost', 'adapter', 'vhost', 'gateway', 'appliance']),
  network: new Set(['infraHost', 'phyNic', 'vswitch', 'vnic']),
  cluster: new Set(['hub', 'cluster', 'acm']),
  app: new Set(['acm']),
}

const objectSummary = computed(() => {
  if (!mockup.value) return ''
  const style = mockup.value.spec.style || 'acm-multi-cluster'
  const clusters = mockup.value.spec.clusters || []
  const n = clusters.length
  const guests = enumerateVHosts(mockup.value).length
  const p = mockup.value.spec.provider || 'libvirt'
  if (style === 'single-sno-ocp') {
    return `Single SNO · adapter ${p} · ${guests} vHosts · OCP-MGMT`
  }
  return `mock-me · adapter ${p} · ${guests} vHosts · OCP-MGMT · ${n} OCP-DEPLOY`
})

const objectRows = computed(() => {
  const m = mockup.value
  if (!m?.spec) return []
  const L = layer.value
  const canvas = m.spec.canvas || {}
  const rows = []

  if (L === 'network') {
    const { nodes: net } = enumerateNetwork(m)
    for (const n of net) {
      rows.push({
        id: n.id,
        kind: n.kind,
        title: n.label,
        sub: n.sub,
      })
    }
    if (L === 'network') return rows
  }

  if (L === 'infra' || L === 'all') {
    const ih = m.spec.infraHost
    if (ih?.id && !canvas.omitHost) {
      rows.push({
        id: ih.id, kind: 'infraHost',
        title: ih.label || 'MACHINE-HOST',
        sub: 'HW host · libvirt / podman',
      })
    }
    if (!canvas.omitHost) {
      rows.push({
        id: 'adapter', kind: 'adapter',
        title: 'ADAPTER',
        sub: `${m.spec.provider || 'libvirt'} IaaS`,
      })
    }
    for (const vh of enumerateVHosts(m)) {
      rows.push({
        id: vh.id, kind: 'vhost',
        title: vh.label,
        sub: vh.sub,
        parentKind: vh.parentKind,
        parentId: vh.parentId,
      })
    }
    const gw = m.spec.gateway
    if (gw?.id && !canvas.omitGateway) {
      rows.push({
        id: gw.id, kind: 'gateway',
        title: gw.label || 'VYOS-GW',
        sub: 'RTR / NF on vHost-GW',
      })
    }
    for (const o of canvas.orphans || []) {
      if (o.kind !== 'appliance') continue
      rows.push({
        id: o.id, kind: 'appliance',
        title: o.label || o.id,
        sub: `${o.applianceType || 'appliance'} · ${o.runsOn || 'unlinked'}`,
      })
    }
  }

  if (L === 'cluster' || L === 'all') {
    if (m.spec.hub?.id && !canvas.omitHub) {
      rows.push({
        id: m.spec.hub.id, kind: 'hub',
        title: m.spec.hub.label || 'MGMT-CLUSTER',
        sub: 'Home cluster · hosts ACM',
      })
    }
    for (const c of m.spec.clusters || []) {
      rows.push({
        id: c.id, kind: 'cluster',
        title: c.label || c.name,
        sub: `Managed OCP · ${c.count || 3} nodes · ${c.phase || 'planned'}`,
      })
    }
  }

  if (L === 'app' || L === 'cluster' || L === 'all') {
    if (m.spec.acm?.id && !canvas.omitACM) {
      rows.push({
        id: m.spec.acm.id, kind: 'acm',
        title: m.spec.acm.label || 'ACM',
        sub: m.spec.acm.enabled ? 'Payload · governs spokes' : 'Disabled',
      })
    }
  }

  if (L === 'all') return rows
  const allow = LAYER_ROWS[L]
  return allow ? rows.filter((r) => allow.has(r.kind)) : rows
})

const selectedNodeData = computed(() => {
  if (!selected.value || !mockup.value) return null
  const k = selected.value.kind
  if (k === 'adapter') {
    return {
      id: 'adapter',
      provider: mockup.value.spec.provider || 'libvirt',
      mode: 'local',
      notes: 'mock-me (podman) talks to this IaaS adapter — local libvirt today; remote/Azure Spot later.',
    }
  }
  if (k === 'vhost') {
    const vh = enumerateVHosts(mockup.value).find((x) => x.id === selected.value.id)
    if (!vh) return null
    if (vh.orphan) {
      return { ...vh, orphan: true, parent: null }
    }
    let parent = null
    if (vh.parentKind === 'gateway') parent = mockup.value.spec.gateway
    else if (vh.parentKind === 'hub') parent = mockup.value.spec.hub
    else parent = mockup.value.spec.clusters.find((c) => c.id === vh.parentId)
    return { ...vh, parent }
  }
  if (k === 'appliance') {
    const o = (mockup.value.spec.canvas?.orphans || []).find((x) => x.id === selected.value.id)
    return o || null
  }
  if (k === 'phyNic' || k === 'vswitch' || k === 'vnic') {
    const { nodes: net } = enumerateNetwork(mockup.value)
    return net.find((x) => x.id === selected.value.id) || null
  }
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
    return { classLabel: '', title: '', roleLine: '', acmLine: '', facts: [], editable: true }
  }
  if (n.kind === 'adapter') {
    return {
      classLabel: 'IaaS adapter',
      title: 'ADAPTER',
      roleLine: 'Role: provision guest vHosts via provider plugin',
      acmLine: 'Fans out to GW / mgmt / deployment vHosts',
      editable: false,
      facts: [
        { k: 'Type', v: d.provider },
        { k: 'Mode', v: d.mode },
        { k: 'Note', v: d.notes },
      ],
    }
  }
  if (n.kind === 'vhost') {
    const p = d.parent || {}
    return {
      classLabel: d.orphan ? 'Free-form vHost' : 'Guest vHost',
      title: d.label,
      roleLine: d.orphan
        ? 'Role: teaching drop — Validate expects a cluster/appliance on top'
        : d.role === 'gw'
          ? 'Role: VM that runs the VyOS RTR / network function'
          : d.role === 'hub'
            ? 'Role: VM that runs the mgmt OCP OS (SNO)'
            : 'Role: VM that backs a deployment OCP node',
      acmLine: 'Infrastructure — OCP/ACM objects live in Cluster / App views',
      editable: !d.orphan,
      orphan: !!d.orphan,
      facts: [
        { k: 'Parent', v: d.orphan ? '(none — orphan)' : (p.label || p.name || d.parentId) },
        { k: 'Size', v: p.cpu ? `${p.cpu}c / ${Math.round((p.memoryMiB || 0) / 1024)}G` : '—' },
        { k: 'Payload', v: d.role === 'gw' ? 'VyOS (RTR)' : (d.orphan ? 'MISSING until linked' : 'OCP node OS') },
      ],
    }
  }
  if (n.kind === 'appliance') {
    return {
      classLabel: 'Appliance / NF',
      title: d.label,
      roleLine: `Role: ${d.applianceType || 'appliance'} payload on a vHost`,
      acmLine: d.runsOn ? `Runs on ${d.runsOn}` : 'Not linked to a vHost — Validate will fail',
      editable: false,
      facts: [
        { k: 'Type', v: d.applianceType || 'other' },
        { k: 'Runs on', v: d.runsOn || '—' },
      ],
    }
  }
  if (n.kind === 'phyNic') {
    return {
      classLabel: 'Physical NIC',
      title: d.label || 'PRI-PHY-NIC',
      roleLine: 'Role: primary bridged uplink on MACHINE-HOST',
      acmLine: 'WAN side of VyOS eth0 attaches here',
      editable: false,
      facts: Object.entries(d.facts || {}).map(([k, v]) => ({ k, v: String(v) })),
    }
  }
  if (n.kind === 'vswitch') {
    return {
      classLabel: 'Lab vSwitch',
      title: d.label,
      roleLine: 'Role: libvirt LAN — guest vNICs + VyOS eth1',
      acmLine: `Gateway ${d.facts?.gateway || '—'} is .1 on this CIDR`,
      editable: false,
      facts: Object.entries(d.facts || {}).map(([k, v]) => ({ k, v: String(v) })),
    }
  }
  if (n.kind === 'vnic') {
    return {
      classLabel: 'Virtual NIC',
      title: d.label,
      roleLine: d.role === 'wan'
        ? 'Role: VyOS WAN — uplinks to PRI-PHY-NIC'
        : d.role === 'lan'
          ? 'Role: VyOS LAN — GW (.1) for the lab CIDR'
          : 'Role: guest NIC on the lab vSwitch',
      acmLine: d.ip ? `IP ${d.ip}` : (d.sub || ''),
      editable: false,
      facts: Object.entries(d.facts || {}).map(([k, v]) => ({ k, v: String(v) })),
    }
  }
  if (n.kind === 'infraHost') {
    return {
      classLabel: 'Host machine',
      title: d.label || 'MACHINE-HOST',
      roleLine: 'Role: HW (or nested) node running libvirtd / podman',
      acmLine: 'Infrastructure',
      editable: true,
      facts: [
        { k: 'SSH', v: `${d.sshUser || 'dasm'}@${d.sshHost || '—'}` },
        { k: 'Inventory', v: d.inventoryRef || '— (link in Edit / Inventory page)' },
        { k: 'OS', v: `${d.os || '—'} · ${d.kind || '—'}` },
        { k: 'Size', v: `${d.cpu}c / ${Math.round((d.memoryMiB || 0) / 1024)}G` },
        { k: 'Disks', v: (d.disks || []).map((x) => `${x.sizeGiB}G ${x.role || ''}`).join(' + ') || `${d.diskGiB}G` },
        { k: 'NICs', v: (d.nics || []).map((x) => x.role || x.mode).join(', ') || '—' },
      ],
    }
  }
  if (n.kind === 'gateway') {
    return {
      classLabel: 'Network function (RTR)',
      title: d.label || 'VYOS-GW',
      roleLine: 'Role: VyOS OS / RTR payload on vHost-GW',
      acmLine: 'Infra picture — sits on the GW vHost from the adapter',
      editable: true,
      facts: [
        { k: 'LAN', v: `${d.lanCIDR || '—'} · ${d.lanIP || ''}` },
        { k: 'WAN', v: d.wanBridge || 'bridged' },
        { k: 'Phase', v: d.phase || 'planned' },
      ],
    }
  }
  if (n.kind === 'hub') {
    return {
      classLabel: 'Home cluster',
      title: d.label || 'MGMT-CLUSTER',
      roleLine: 'Role: OCP that hosts ACM (ACM does not fully self-manage yet)',
      acmLine: 'Cluster mgmt · sovereign / multi-tenant story later',
      editable: true,
      facts: [
        { k: 'Hostname', v: d.hostname || '—' },
        { k: 'Profile', v: `${d.profile || '—'} · OCP ${d.version || '—'}` },
        { k: 'Size', v: `${d.cpu}c / ${Math.round((d.memoryMiB || 0) / 1024)}G / ${d.diskGiB}G` },
        { k: 'vHosts', v: '1 (SNO)' },
      ],
    }
  }
  if (n.kind === 'acm') {
    return {
      classLabel: 'Application payload',
      title: d.label || 'ACM',
      roleLine: 'Role: MultiClusterHub on mgmt — governs deployments',
      acmLine: d.enabled
        ? 'Today: lives on mgmt, manages spokes · self-mgmt later'
        : 'Disabled',
      editable: true,
      facts: [
        { k: 'MCE', v: d.mceChannel || '—' },
        { k: 'ACM', v: d.acmChannel || '—' },
      ],
    }
  }
  return {
    classLabel: 'Managed cluster',
    title: d.label || d.name,
    roleLine: 'Role: Spoke OCP · lifecycle owned by ACM',
    acmLine: 'Cluster mgmt · backed by guest vHosts in Infrastructure',
    editable: true,
    facts: [
      { k: 'Name', v: d.name || '—' },
      { k: 'Phase', v: d.phase || 'planned' },
      { k: 'vHosts', v: `${d.count || 3}× ${d.cpu}c / ${Math.round((d.memoryMiB || 0) / 1024)}G` },
      { k: 'API VIP', v: d.apiVIP || '—' },
    ],
  }
})

async function load() {
  loading.value = true
  try {
    const [m, inv] = await Promise.all([
      getMockup(props.id),
      listInventory().catch(() => []),
    ])
    mockup.value = m
    inventory.value = inv || []
    canvasMode.value = mockup.value.spec.canvasMode || 'guided'
    if (!layerOptions.value.some((o) => o.id === layer.value)) {
      layer.value = 'all'
    }
  } catch (e) {
    Notify.create({ type: 'negative', message: e.message })
  } finally {
    loading.value = false
  }
}

async function onCanvasMode(mode) {
  if (!mockup.value) return
  mockup.value.spec.canvasMode = mode
  const c = ensureCanvas(mockup.value)
  if (mode === 'freeform' && c.showRelations === undefined) {
    c.showRelations = false
  }
  await persistQuiet()
  Notify.create({
    type: 'info',
    message: mode === 'freeform'
      ? 'Free-form: no constrained edges — drop objects and Validate. Promote→Guided is not supported yet.'
      : 'Guided rack mode (constrained picture).',
  })
}

async function persistQuiet() {
  try {
    mockup.value = await saveMockup(props.id, mockup.value)
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  }
}

function onAddOrphanVHost() {
  const c = ensureCanvas(mockup.value)
  const id = newOrphanId('ff-vhost')
  const node = { id, kind: 'vhost', label: `vHost-${c.orphans.filter((o) => o.kind === 'vhost').length + 1}`, x: 400, y: 320 }
  c.orphans.push(node)
  if (!mockup.value.layout.nodes) mockup.value.layout.nodes = {}
  mockup.value.layout.nodes[id] = { x: node.x, y: node.y }
  selected.value = { id, kind: 'vhost' }
  persist()
}

function onAddAppliance(type) {
  const c = ensureCanvas(mockup.value)
  const vhosts = enumerateVHosts(mockup.value)
  const orphanVhs = vhosts.filter((v) => v.orphan)
  const runsOn = orphanVhs[0]?.id || vhosts[0]?.id || ''
  const id = newOrphanId('ff-app')
  const label = type === 'haproxy' ? 'HAProxy' : 'Appliance'
  const node = {
    id, kind: 'appliance', label, applianceType: type, runsOn,
    x: 400, y: 240,
  }
  c.orphans.push(node)
  if (!mockup.value.layout.nodes) mockup.value.layout.nodes = {}
  mockup.value.layout.nodes[id] = { x: node.x, y: node.y }
  selected.value = { id, kind: 'appliance' }
  persist()
  if (!runsOn) {
    Notify.create({ type: 'warning', message: 'No vHost to sit on — Validate will flag this appliance.' })
  }
}

function toggleOmit(key) {
  const c = ensureCanvas(mockup.value)
  c[key] = !c[key]
  persist()
}

function toggleShowRelations() {
  const c = ensureCanvas(mockup.value)
  c.showRelations = !c.showRelations
  persist()
}

function stripForTeaching() {
  Dialog.create({
    title: 'Strip to blank teaching canvas?',
    message: 'Hides host/gateway/mgmt/ACM, clears deployment clusters and free-form orphans. YAML keep-alives remain for undo via Show toggles (except cleared clusters/orphans).',
    cancel: true,
    persistent: true,
  }).onOk(async () => {
    const c = ensureCanvas(mockup.value)
    c.omitHost = true
    c.omitGateway = true
    c.omitHub = true
    c.omitACM = true
    c.orphans = []
    c.showRelations = false
    mockup.value.spec.clusters = []
    selected.value = null
    await persist()
  })
}

function onRemoveOrphan(id) {
  const c = ensureCanvas(mockup.value)
  c.orphans = c.orphans.filter((o) => o.id !== id)
  if (mockup.value.layout?.nodes) delete mockup.value.layout.nodes[id]
  if (selected.value?.id === id) selected.value = null
  persist()
}

async function onValidate() {
  validating.value = true
  validateResult.value = null
  try {
    await persistQuiet()
    // Free-form: topology teaching check on in-memory canvas.
    // Guided: ValidatePlan (auto-derive if needed) and advance phase → validated.
    if (isFreeForm.value) {
      validateResult.value = await validateMockup(props.id, mockup.value)
    } else {
      validateResult.value = await validateMockup(props.id)
      if (validateResult.value.mockup) mockup.value = validateResult.value.mockup
    }
    validateOpen.value = true
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    validating.value = false
  }
}

async function onDeploy() {
  deploying.value = true
  deployJob.value = null
  try {
    await persistQuiet()
    const res = await deployMockup(props.id)
    if (res.mockup) mockup.value = res.mockup
    deployJob.value = res.job
    deployOpen.value = true
  } catch (e) {
    const data = e.response?.data
    const msg = typeof data === 'string' ? data : (data?.error || e.message)
    Notify.create({ type: 'negative', message: msg, timeout: 7000 })
    if (data?.mockup) mockup.value = data.mockup
  } finally {
    deploying.value = false
  }
}

async function onDeployFinished(data) {
  if (data?.mockup) mockup.value = data.mockup
  else {
    try {
      mockup.value = await getMockup(props.id)
    } catch { /* ignore */ }
  }
}

async function onClean() {
  cleaning.value = true
  try {
    const res = await cleanMockup(props.id)
    if (res.mockup) mockup.value = res.mockup
    Notify.create({
      type: 'positive',
      message: res.message || 'Cleaned — Validate/Deploy unlocked',
      timeout: 5000,
    })
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    cleaning.value = false
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
  if (!d || !selected.value || selected.value.kind === 'adapter') return
  // vHosts edit their parent object (gateway / hub / cluster)
  if (selected.value.kind === 'vhost') {
    if (!d.parent) return
    editKind.value = d.parentKind
    editNode.value = { ...d.parent }
    editOpen.value = true
    return
  }
  editKind.value = selected.value.kind
  editNode.value = { ...d }
  editOpen.value = true
}

function onMove({ id, x, y }) {
  if (!mockup.value.layout.nodes) mockup.value.layout.nodes = {}
  mockup.value.layout.nodes[id] = { x, y }
  const orphan = mockup.value.spec.canvas?.orphans?.find((o) => o.id === id)
  if (orphan) {
    orphan.x = x
    orphan.y = y
  }
  clearTimeout(layoutTimer)
  layoutTimer = setTimeout(async () => {
    try {
      await patchLayout(props.id, mockup.value.layout)
      if (orphan) await persistQuiet()
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
.swatch-vhost { background: #78909c; }
.swatch-gw { background: #c62828; }
.swatch-phy { background: #455a64; }
.swatch-vswitch { background: #ef6c00; }
.swatch-vnic { background: #ffa726; }
.swatch-mgmt { background: #1a237e; }
.swatch-dep { background: #1565c0; }
.swatch-acm { background: #00838f; }

.layer-bar {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 0.5rem;
}
@media (max-width: 1100px) {
  .layer-bar { grid-template-columns: 1fr 1fr; }
}
.layer-tab {
  text-align: left;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  background: #fff;
  padding: 0.55rem 0.75rem;
  cursor: pointer;
}
.layer-tab:hover { border-color: #90caf9; }
.layer-tab.active {
  border-color: #1565c0;
  background: #e3f2fd;
}
.layer-tab-title {
  display: block;
  font-size: 0.85rem;
  font-weight: 700;
  color: #263238;
}
.layer-tab-sub {
  display: block;
  margin-top: 0.1rem;
  font-size: 0.72rem;
  color: #90a4ae;
}

.dot-adapter { background: #546e7a; }

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
.dot-adapter { background: #546e7a; }
.dot-appliance { background: #6d4c41; }
.dot-vhost { background: #78909c; }
.dot-phyNic { background: #455a64; }
.dot-vswitch { background: #ef6c00; }
.dot-vnic { background: #ffa726; }
.dot-gateway { background: #c62828; }
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
