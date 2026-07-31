<template>
  <q-page padding class="wide">
    <div class="row items-center q-mb-md">
      <q-btn flat dense icon="arrow_back" :to="{ name: 'mockups' }" />
      <div class="text-h4 q-ml-sm">Topology</div>
      <q-space />
      <q-btn outline color="primary" icon="add" label="Add cluster" class="q-mr-sm" @click="onAddCluster" />
      <q-btn color="primary" icon="save" label="Save" :loading="saving" @click="persist" />
    </div>

    <div v-if="loading" class="row justify-center q-my-xl"><q-spinner size="3em" color="primary" /></div>
    <template v-else-if="mockup">
      <p class="text-body2 text-grey-8 q-mb-md">
        {{ mockup.metadata.name }} · Drag nodes to arrange · Click a node to edit
        (MGMT-CLUSTER / ACM / DEPLOYMENT-CLUSTER). Edges: hub → ACM → clusters.
      </p>

      <div class="row q-col-gutter-md">
        <div class="col-12 col-md-8">
          <TopologyCanvas
            :mockup="mockup"
            :selected-id="selected?.id"
            @select="onSelect"
            @move="onMove"
          />
        </div>
        <div class="col-12 col-md-4">
          <q-card flat bordered>
            <q-card-section>
              <div class="text-h6">Lab network</div>
              <q-input v-model="mockup.spec.baseDomain" outlined dense label="Base domain" class="q-mb-sm" />
              <q-input v-model="mockup.spec.network.machineCIDR" outlined dense label="Machine CIDR" class="q-mb-sm" />
              <q-input v-model="mockup.spec.network.gateway" outlined dense label="Gateway" class="q-mb-sm" />
              <q-input v-model="mockup.spec.network.apiVIP" outlined dense label="API VIP" class="q-mb-sm" />
              <q-input v-model="mockup.spec.network.ingressVIP" outlined dense label="Ingress VIP" />
            </q-card-section>
            <q-separator />
            <q-card-actions vertical>
              <q-btn flat color="primary" icon="playlist_play" label="Open wizard"
                :to="{ name: 'wizard', params: { id } }" />
              <q-btn flat color="primary" icon="description" label="Derive YAML" :loading="deriving" @click="onDerive" />
            </q-card-actions>
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
import { onMounted, ref } from 'vue'
import { Notify } from 'quasar'
import TopologyCanvas from 'src/components/TopologyCanvas.vue'
import NodeEditDialog from 'src/components/NodeEditDialog.vue'
import { getMockup, saveMockup, patchLayout, addCluster, deriveMockup } from 'src/services/api'

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
  if (n.kind === 'hub') editNode.value = { ...mockup.value.spec.hub }
  else if (n.kind === 'acm') editNode.value = { ...mockup.value.spec.acm }
  else {
    const c = mockup.value.spec.clusters.find((x) => x.id === n.id)
    editNode.value = { ...c }
  }
  editOpen.value = true
}

function onMove({ id, x, y }) {
  if (!mockup.value.layout.nodes) mockup.value.layout.nodes = {}
  mockup.value.layout.nodes[id] = { x, y }
  clearTimeout(layoutTimer)
  layoutTimer = setTimeout(async () => {
    try {
      await patchLayout(props.id, mockup.value.layout)
    } catch { /* ignore transient */ }
  }, 400)
}

async function onSaveNode({ kind, node }) {
  if (kind === 'hub') mockup.value.spec.hub = { ...mockup.value.spec.hub, ...node }
  else if (kind === 'acm') mockup.value.spec.acm = { ...mockup.value.spec.acm, ...node }
  else {
    const i = mockup.value.spec.clusters.findIndex((c) => c.id === node.id)
    if (i >= 0) mockup.value.spec.clusters[i] = { ...mockup.value.spec.clusters[i], ...node }
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
