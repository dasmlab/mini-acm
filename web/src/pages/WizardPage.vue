<template>
  <q-page padding class="wide">
    <div class="row items-center q-mb-md">
      <q-btn flat dense icon="arrow_back" :to="{ name: 'topology', params: { id } }" />
      <div class="col">
        <div class="text-h4">Wizard</div>
        <div class="text-caption text-grey-7" v-if="mockup">
          {{ mockup.metadata.name }} — fill MVP gaps, then derive YAML for CLI
        </div>
      </div>
      <q-btn outline color="primary" label="Topology" :to="{ name: 'topology', params: { id } }" class="q-mr-sm" />
      <q-btn color="primary" icon="save" label="Save gaps" :loading="saving" @click="persist" />
    </div>

    <div v-if="loading" class="row justify-center q-my-xl"><q-spinner size="3em" color="primary" /></div>
    <template v-else-if="mockup">
      <q-banner v-if="gapSummary.length" dense rounded class="bg-orange-1 text-orange-10 q-mb-md">
        <template #avatar><q-icon name="warning" color="orange-9" /></template>
        Still needed: {{ gapSummary.join(' · ') }}
      </q-banner>

      <q-stepper v-model="step" color="primary" animated flat bordered header-nav class="wizard-stepper">
        <q-step :name="0" title="Infra host" icon="precision_manufacturing" :done="step > 0">
          <div class="text-subtitle1 q-mb-sm">
            Phase 0 — MACHINE-HOST (libvirtd) + VYOS-GW (edge)
          </div>
          <div class="row q-col-gutter-md">
            <div class="col-12 col-md-6" v-if="mockup.spec.infraHost">
              <q-banner dense rounded class="bg-blue-grey-1 text-blue-grey-10 q-mb-md">
                RHEL MACHINE-HOST at <code>{{ mockup.spec.infraHost.sshHost || 'ssh?' }}</code>.
                Bridged = uplink; VMnet12 host-only optional. 250G = OS/logs; 400G = guest pool.
                Lab guests are <em>not</em> on VMnet12 — they sit on libvirt LAN behind VyOS.
              </q-banner>
              <q-markup-table flat bordered dense>
                <tbody>
                  <tr><td>Label</td><td>{{ mockup.spec.infraHost.label }}</td></tr>
                  <tr><td>Hostname</td><td>{{ mockup.spec.infraHost.hostname }}</td></tr>
                  <tr><td>SSH</td><td>{{ mockup.spec.infraHost.sshHost }}</td></tr>
                  <tr><td>Kind / OS</td><td>{{ mockup.spec.infraHost.kind }} / {{ mockup.spec.infraHost.hypervisor }} · {{ mockup.spec.infraHost.os }}</td></tr>
                  <tr><td>Disks</td><td>{{ (mockup.spec.infraHost.disks || []).map(d => `${d.sizeGiB}G ${d.role}`).join(' + ') }}</td></tr>
                  <tr><td>NICs</td><td>{{ (mockup.spec.infraHost.nics || []).map(n => `${n.name} ${n.role||n.mode}`).join(' · ') }}</td></tr>
                </tbody>
              </q-markup-table>
              <q-input v-model="mockup.spec.infraHost.sshHost" outlined dense label="SSH endpoint" class="q-mt-md" />
            </div>
            <div class="col-12 col-md-6" v-if="mockup.spec.gateway">
              <q-banner dense rounded class="bg-orange-1 text-orange-10 q-mb-md">
                <strong>VYOS-GW</strong> — boot ISO later. eth0 WAN (bridged), eth1 LAN
                (<code>{{ mockup.spec.gateway.lanCIDR }}</code>). Hub + deployment VMs on LAN.
              </q-banner>
              <q-markup-table flat bordered dense>
                <tbody>
                  <tr><td>Hostname</td><td>{{ mockup.spec.gateway.hostname }}</td></tr>
                  <tr><td>Phase</td><td>{{ mockup.spec.gateway.phase }}</td></tr>
                  <tr><td>WAN</td><td>{{ mockup.spec.gateway.wanBridge }}</td></tr>
                  <tr><td>LAN</td><td>{{ mockup.spec.gateway.lanNetwork }} · {{ mockup.spec.gateway.lanIP }} / {{ mockup.spec.gateway.lanCIDR }}</td></tr>
                  <tr><td>NAT / FW</td><td>{{ mockup.spec.gateway.nat ? 'yes' : 'no' }} / {{ mockup.spec.gateway.firewall ? 'yes' : 'no' }}</td></tr>
                </tbody>
              </q-markup-table>
              <q-input v-model="mockup.spec.gateway.isoPath" outlined dense label="VyOS ISO path (MVP gap)" class="q-mt-md" />
              <q-banner dense rounded class="bg-grey-2 text-caption q-mt-sm">
                Derive emits <code>out/infra-host.yaml</code> + <code>out/gateway.yaml</code>.
              </q-banner>
            </div>
          </div>
          <q-stepper-navigation>
            <q-btn color="primary" label="Continue" @click="step = 1" />
          </q-stepper-navigation>
        </q-step>

        <q-step :name="1" title="Hub create" icon="dns" :done="step > 1">
          <div class="text-subtitle1 q-mb-sm">Local Agent-based ISO → SNO (OCP only, not ACM)</div>
          <div class="row q-col-gutter-md">
            <div class="col-12 col-md-5">
              <q-markup-table flat bordered dense>
                <tbody>
                  <tr><td>Hostname</td><td>{{ mockup.spec.hub.hostname }}</td></tr>
                  <tr><td>Profile / size</td><td>{{ mockup.spec.hub.profile }} · {{ mockup.spec.hub.cpu }}c / {{ mockup.spec.hub.memoryMiB }}MiB / {{ mockup.spec.hub.diskGiB }}GiB</td></tr>
                  <tr><td>Version</td><td>{{ mockup.spec.hub.version }}</td></tr>
                  <tr><td>Runs on</td><td>{{ mockup.spec.infraHost?.hostname || 'INFRA-HOST' }} (libvirt guest)</td></tr>
                  <tr><td>ISO</td><td><code>openshift-install agent create image</code></td></tr>
                </tbody>
              </q-markup-table>
            </div>
            <div class="col-12 col-md-7">
              <q-input v-model="mockup.spec.gaps.pullSecretFile" outlined dense label="Pull secret file path" class="q-mb-sm"
                hint="Required — $PULL_SECRET_FILE or absolute path" />
              <q-input v-model="mockup.spec.gaps.sshPublicKeyFile" outlined dense label="SSH public key path" class="q-mb-md"
                hint="Required — $SSH_PUBLIC_KEY_FILE or ~/.ssh/id_ed25519.pub" />
              <q-banner class="bg-grey-2" rounded dense>
                <code>mini-mock --manual hub create --config data/mockups/{{ id }}/out/hub.yaml --skip-wait --skip-acm</code>
              </q-banner>
            </div>
          </div>
          <q-stepper-navigation>
            <q-btn flat label="Back" @click="step = 0" class="q-mr-sm" />
            <q-btn color="primary" label="Continue" @click="step = 2" />
          </q-stepper-navigation>
        </q-step>

        <q-step :name="2" title="Install ACM" icon="extension" :done="step > 2">
          <div class="text-subtitle1 q-mb-sm">Operators on the live hub — no second ISO</div>
          <div class="row q-col-gutter-md">
            <div class="col-12 col-md-6">
              <q-toggle v-model="mockup.spec.hub.installACM" label="Install ACM after hub" class="q-mb-sm" />
              <q-input v-model="mockup.spec.acm.mceChannel" outlined dense label="MCE channel" class="q-mb-sm" />
              <q-input v-model="mockup.spec.acm.acmChannel" outlined dense label="ACM channel" class="q-mb-sm" />
            </div>
            <div class="col-12 col-md-6">
              <q-input v-model="mockup.spec.gaps.hubKubeconfig" outlined dense label="Hub kubeconfig path" class="q-mb-md"
                :hint="`Suggested: ./data/hub-${mockup.metadata.name}/auth/kubeconfig`" />
              <q-banner class="bg-grey-2" rounded dense>
                <code>mini-mock hub install-acm --config data/mockups/{{ id }}/out/hub.yaml</code>
              </q-banner>
            </div>
          </div>
          <q-stepper-navigation>
            <q-btn flat label="Back" @click="step = 1" class="q-mr-sm" />
            <q-btn color="primary" label="Continue" @click="step = 3" />
          </q-stepper-navigation>
        </q-step>

        <q-step :name="3" title="Cluster create" icon="developer_board" :done="step > 3">
          <div class="row items-center q-mb-sm">
            <div class="text-subtitle1">
              Each DEPLOYMENT-CLUSTER → 3 VMs + DNS/HAProxy + ACM CRs (no ISO yet)
            </div>
            <q-space />
            <q-btn outline dense color="primary" icon="add" label="Add cluster" @click="onAddCluster" />
          </div>

          <div class="row q-col-gutter-md">
            <div v-for="(c, idx) in mockup.spec.clusters" :key="c.id" class="col-12 col-md-6">
              <q-card flat bordered class="cluster-card full-height">
                <q-card-section class="row items-start">
                  <div>
                    <div class="text-subtitle2">{{ c.label }}</div>
                    <div class="text-caption text-grey-7">
                      {{ c.name }} · {{ c.count }}× {{ c.cpu }}c / {{ c.memoryMiB }}MiB / {{ c.diskGiB }}GiB · {{ c.profile }}
                    </div>
                  </div>
                  <q-space />
                  <q-badge :color="phaseColor(c.phase)" class="text-capitalize">{{ c.phase || 'planned' }}</q-badge>
                </q-card-section>
                <q-card-section class="q-pt-none">
                  <q-select
                    v-model="c.phase"
                    :options="phaseOptions"
                    outlined dense label="Lifecycle phase"
                    class="q-mb-sm"
                  />
                  <q-input v-model="c.clusterImageSet" outlined dense label="ClusterImageSet name" class="q-mb-sm"
                    :hint="`MVP gap — e.g. ${imageSetName(c.version)}`"
                    @blur="ensureImageSet(c)" />
                  <q-input v-model="c.apiVIP" outlined dense label="API VIP" class="q-mb-sm" />
                  <q-input v-model="c.ingressVIP" outlined dense label="Ingress VIP" class="q-mb-sm" />
                  <q-input v-model="c.ipBase" outlined dense label="Node IP base" class="q-mb-sm" />
                  <q-banner dense rounded class="bg-grey-2 text-caption">
                    <code>mini-mock --manual cluster create --config …/out/cluster-{{ c.name }}.yaml</code>
                  </q-banner>
                </q-card-section>
                <q-card-actions align="right">
                  <q-btn flat dense color="negative" label="Remove" icon="delete"
                    :disable="mockup.spec.clusters.length <= 1"
                    @click="onDeleteCluster(c)" />
                  <q-btn flat dense color="primary" label="Edit sizes"
                    :to="{ name: 'topology', params: { id } }" />
                </q-card-actions>
              </q-card>
              <div v-if="idx === mockup.spec.clusters.length - 1" class="q-mt-sm text-caption text-grey-6">
                Tip: use Topology → Add cluster for another lifecycle object with its own VIPs/MACs.
              </div>
            </div>
          </div>

          <q-toggle v-model="mockup.spec.gaps.manualApprove" label="Manual Agent approve / bind (all clusters)" class="q-mt-md" />

          <q-stepper-navigation>
            <q-btn flat label="Back" @click="step = 2" class="q-mr-sm" />
            <q-btn color="primary" label="Continue" @click="step = 4" />
          </q-stepper-navigation>
        </q-step>

        <q-step :name="4" title="Attach ISO" icon="album">
          <div class="text-subtitle1 q-mb-sm">Per-cluster discovery ISO → Agents → install</div>
          <div class="row q-col-gutter-md">
            <div v-for="c in mockup.spec.clusters" :key="'iso-' + c.id" class="col-12 col-md-6">
              <q-card flat bordered>
                <q-card-section>
                  <div class="text-subtitle2">{{ c.label }} ({{ c.name }})</div>
                  <q-input v-model="c.discoveryISO" outlined dense label="Discovery ISO path" class="q-mt-sm"
                    :hint="`MVP gap — InfraEnv status.isoDownloadURL → ./discovery-${c.name}.iso`" />
                  <q-banner dense rounded class="bg-grey-2 q-mt-sm text-caption">
                    <code>mini-mock cluster attach-iso --config …/cluster-{{ c.name }}.yaml --iso {{ c.discoveryISO || `./discovery-${c.name}.iso` }}</code>
                  </q-banner>
                </q-card-section>
              </q-card>
            </div>
          </div>
          <q-stepper-navigation>
            <q-btn flat label="Back" @click="step = 3" class="q-mr-sm" />
            <q-btn color="primary" icon="description" label="Derive YAML" :loading="deriving" @click="onDerive" />
          </q-stepper-navigation>
        </q-step>
      </q-stepper>
    </template>
  </q-page>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { Dialog, Notify } from 'quasar'
import {
  getMockup, saveMockup, deriveMockup, addCluster, deleteCluster, imageSetName,
} from 'src/services/api'

const props = defineProps({ id: { type: String, required: true } })

const mockup = ref(null)
const loading = ref(true)
const saving = ref(false)
const deriving = ref(false)
const step = ref(0)
const phaseOptions = ['planned', 'created', 'installing', 'ready', 'destroy']

const gapSummary = computed(() => {
  if (!mockup.value) return []
  const g = mockup.value.spec.gaps || {}
  const missing = []
  if (!g.pullSecretFile || g.pullSecretFile.startsWith('$')) missing.push('pull secret path')
  if (!g.sshPublicKeyFile || g.sshPublicKeyFile.startsWith('$')) missing.push('SSH key path')
  for (const c of mockup.value.spec.clusters || []) {
    if (!c.clusterImageSet) missing.push(`${c.name} ClusterImageSet`)
    if (step.value >= 4 && !c.discoveryISO) missing.push(`${c.name} discovery ISO`)
  }
  return missing
})

function phaseColor(phase) {
  return {
    planned: 'grey-6',
    created: 'blue-7',
    installing: 'orange-7',
    ready: 'positive',
    destroy: 'negative',
  }[phase] || 'grey-6'
}

function ensureImageSet(c) {
  if (!c.clusterImageSet) c.clusterImageSet = imageSetName(c.version)
}

async function load() {
  loading.value = true
  try {
    mockup.value = await getMockup(props.id)
    for (const c of mockup.value.spec.clusters || []) {
      ensureImageSet(c)
    }
  } catch (e) {
    Notify.create({ type: 'negative', message: e.message })
  } finally {
    loading.value = false
  }
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
    message: `Removes lifecycle object ${c.label} (${c.name}) from this MockUp.`,
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
    Notify.create({
      type: 'positive',
      message: `Derived YAML: ${Object.values(res.paths || {}).join(', ')}`,
      timeout: 6000,
    })
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    deriving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.wizard-stepper {
  max-width: 100%;
}
.cluster-card {
  min-height: 100%;
}
.full-height {
  height: 100%;
}
</style>
