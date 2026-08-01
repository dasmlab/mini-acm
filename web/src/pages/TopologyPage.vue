<template>
  <q-page padding class="wide">
    <div class="row items-center q-mb-md">
      <q-btn flat dense icon="arrow_back" :to="{ name: 'mockups' }" />
      <div class="col">
        <div class="text-h4">Topology</div>
        <div class="text-caption text-grey-7" v-if="mockup">
          {{ mockup.metadata.name }} · MACHINE-HOST + VYOS-GW ·
          {{ mockup.spec.clusters.length }} deployment cluster(s) on lab LAN
        </div>
      </div>
      <q-btn outline color="primary" icon="add" label="Add cluster" class="q-mr-sm" @click="onAddCluster" />
      <q-btn color="primary" icon="save" label="Save" :loading="saving" @click="persist" />
    </div>

    <div v-if="loading" class="row justify-center q-my-xl"><q-spinner size="3em" color="primary" /></div>
    <template v-else-if="mockup">
      <p class="text-body2 text-grey-8 q-mb-md">
        <strong>MACHINE-HOST</strong> (libvirtd) → boots <strong>VYOS-GW</strong> (WAN bridge / LAN obscure private)
        + MGMT-CLUSTER + DEPLOYMENT-CLUSTER guests on the LAN.
        ACM on hub orchestrates deployment LCs. Dashed = hosts; orange = LAN; grey = ACM manage.
      </p>

      <div class="row q-col-gutter-md">
        <div class="col-12 col-lg-8">
          <TopologyCanvas
            :mockup="mockup"
            :selected-id="selected?.id"
            @select="onSelect"
            @move="onMove"
          />
        </div>
        <div class="col-12 col-lg-4">
          <q-card flat bordered class="q-mb-md" v-if="mockup.spec.infraHost">
            <q-card-section class="row items-center">
              <div>
                <div class="text-subtitle1">{{ mockup.spec.infraHost.label || 'MACHINE-HOST' }}</div>
                <div class="text-caption text-grey-7">
                  {{ mockup.spec.infraHost.hostname }} · ssh {{ mockup.spec.infraHost.sshHost || '—' }} ·
                  {{ mockup.spec.infraHost.os }} · {{ mockup.spec.infraHost.kind }}
                </div>
              </div>
              <q-space />
              <q-btn flat dense round icon="edit" @click="editInfra" />
            </q-card-section>
            <q-card-section class="q-pt-none text-caption text-grey-8">
              {{ mockup.spec.infraHost.cpu }}c /
              {{ Math.round(mockup.spec.infraHost.memoryMiB / 1024) }}G ·
              {{ (mockup.spec.infraHost.disks || []).map(d => d.sizeGiB + 'G ' + (d.role || '')).join(' + ') }} ·
              {{ (mockup.spec.infraHost.nics || []).map(n => (n.role || n.mode)).join(', ') }}.
              Guests: {{ guestVMCount }} VMs (+ VyOS).
            </q-card-section>
          </q-card>

          <q-card flat bordered class="q-mb-md" v-if="mockup.spec.gateway">
            <q-card-section class="row items-center">
              <div>
                <div class="text-subtitle1">{{ mockup.spec.gateway.label || 'VYOS-GW' }}</div>
                <div class="text-caption text-grey-7">
                  {{ mockup.spec.gateway.hostname }} · {{ mockup.spec.gateway.phase || 'planned' }} ·
                  LAN {{ mockup.spec.gateway.lanCIDR }}
                </div>
              </div>
              <q-space />
              <q-btn flat dense round icon="edit" @click="editGateway" />
            </q-card-section>
            <q-card-section class="q-pt-none text-caption text-grey-8">
              eth0 WAN ({{ mockup.spec.gateway.wanBridge }}) · eth1 LAN ({{ mockup.spec.gateway.lanNetwork }}
              {{ mockup.spec.gateway.lanIP }}) · NAT {{ mockup.spec.gateway.nat ? 'on' : 'off' }}.
              Installer TBD — object is inventory for now.
            </q-card-section>
          </q-card>

          <q-card flat bordered class="q-mb-md">
            <q-card-section>
              <div class="text-subtitle1">Lab network (behind VyOS)</div>
              <q-input v-model="mockup.spec.baseDomain" outlined dense label="Base domain" class="q-mb-sm" />
              <q-input v-model="mockup.spec.network.machineCIDR" outlined dense label="Machine CIDR" class="q-mb-sm" />
              <q-input v-model="mockup.spec.network.gateway" outlined dense label="Gateway (VyOS LAN IP)" class="q-mb-sm" />
              <div class="text-caption text-grey-7 q-mb-xs">Hub defaults (per-cluster VIPs on each object)</div>
              <q-input v-model="mockup.spec.network.apiVIP" outlined dense label="Default API VIP" class="q-mb-sm" />
              <q-input v-model="mockup.spec.network.ingressVIP" outlined dense label="Default Ingress VIP" />
            </q-card-section>
            <q-separator />
            <q-card-actions vertical>
              <q-btn flat color="primary" icon="playlist_play" label="Open wizard"
                :to="{ name: 'wizard', params: { id } }" />
              <q-btn flat color="primary" icon="description" label="Derive YAML" :loading="deriving" @click="onDerive" />
            </q-card-actions>
          </q-card>

          <q-card flat bordered>
            <q-card-section class="row items-center">
              <div class="text-subtitle1">Deployment clusters</div>
              <q-space />
              <q-btn flat dense round icon="add" color="primary" @click="onAddCluster" />
            </q-card-section>
            <q-list separator>
              <q-item v-for="c in mockup.spec.clusters" :key="c.id" clickable v-ripple @click="editCluster(c)">
                <q-item-section>
                  <q-item-label>{{ c.label }}</q-item-label>
                  <q-item-label caption>
                    {{ c.name }} · {{ c.count }}×{{ c.cpu }}c/{{ Math.round(c.memoryMiB / 1024) }}G · {{ c.phase || 'planned' }}
                  </q-item-label>
                </q-item-section>
                <q-item-section side>
                  <div class="row no-wrap q-gutter-xs">
                    <q-btn flat dense round icon="edit" size="sm" @click.stop="editCluster(c)" />
                    <q-btn flat dense round icon="delete" size="sm" color="negative"
                      :disable="mockup.spec.clusters.length <= 1"
                      @click.stop="onDeleteCluster(c)" />
                  </div>
                </q-item-section>
              </q-item>
            </q-list>
          </q-card>
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

const guestVMCount = computed(() => {
  const clusters = mockup.value?.spec?.clusters || []
  return clusters.reduce((n, c) => n + (c.count || 3), 0) + 1 // + hub SNO (VyOS counted separately)
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
  selected.value = n
  editKind.value = n.kind
  if (n.kind === 'infraHost') editNode.value = { ...mockup.value.spec.infraHost }
  else if (n.kind === 'gateway') editNode.value = { ...mockup.value.spec.gateway }
  else if (n.kind === 'hub') editNode.value = { ...mockup.value.spec.hub }
  else if (n.kind === 'acm') editNode.value = { ...mockup.value.spec.acm }
  else {
    const c = mockup.value.spec.clusters.find((x) => x.id === n.id)
    editNode.value = { ...c }
  }
  editOpen.value = true
}

function editInfra() {
  selected.value = { id: mockup.value.spec.infraHost.id, kind: 'infraHost' }
  editKind.value = 'infraHost'
  editNode.value = { ...mockup.value.spec.infraHost }
  editOpen.value = true
}

function editGateway() {
  selected.value = { id: mockup.value.spec.gateway.id, kind: 'gateway' }
  editKind.value = 'gateway'
  editNode.value = { ...mockup.value.spec.gateway }
  editOpen.value = true
}

function editCluster(c) {
  selected.value = { id: c.id, kind: 'cluster' }
  editKind.value = 'cluster'
  editNode.value = { ...c }
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
    Notify.create({ type: 'positive', message: `Added ${res.cluster.label}` })
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  }
}

function onDeleteCluster(c) {
  Dialog.create({
    title: 'Remove deployment cluster?',
    message: `Removes ${c.label} (${c.name}) from this MockUp — the cluster lifecycle object and its planned VMs.`,
    cancel: true,
    persistent: true,
  }).onOk(async () => {
    try {
      mockup.value = await deleteCluster(props.id, c.id)
      Notify.create({ type: 'positive', message: `Removed ${c.label}` })
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
