<template>
  <q-dialog :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)" persistent>
    <q-card style="min-width: 520px; max-width: 640px">
      <q-card-section class="row items-center">
        <div class="text-h6">{{ title }}</div>
        <q-space />
        <q-btn flat round dense icon="close" v-close-popup />
      </q-card-section>

      <q-card-section v-if="kind === 'infraHost'">
        <q-banner dense rounded class="bg-blue-grey-1 text-blue-grey-10 q-mb-md">
          <strong>MACHINE-HOST</strong> — RHEL where libvirtd runs (BM or nested).
          Bridged NIC = SSH/uplink; host-only (VMnet12) optional. Guest VMs + VyOS live on libvirt LAN behind VYOS-GW.
          Large disk = pool for guests; small disk = OS/logs.
        </q-banner>
        <q-input v-model="draft.label" outlined dense label="Label" class="q-mb-sm" />
        <q-input v-model="draft.hostname" outlined dense label="Hostname" class="q-mb-sm" />
        <q-select v-model="draft.kind" :options="infraKinds" outlined dense label="Host kind"
          class="q-mb-sm" hint="nested-vm = VMware/KVM outer VM with nested virt" />
        <q-select v-if="draft.kind === 'nested-vm'" v-model="draft.hypervisor" :options="hypervisors"
          outlined dense label="Outer hypervisor" class="q-mb-sm" />
        <q-select v-model="draft.os" :options="infraOS" outlined dense label="OS" class="q-mb-sm" />
        <q-input v-model="draft.libvirtURI" outlined dense label="Libvirt URI" class="q-mb-sm" />
        <q-input v-model="draft.networkName" outlined dense label="Lab libvirt net (behind VyOS)" class="q-mb-sm" />
        <q-input v-model="draft.storagePool" outlined dense label="Storage pool" class="q-mb-sm" />
        <q-input v-model="draft.sshHost" outlined dense label="SSH endpoint" class="q-mb-sm"
          hint="e.g. 192.168.1.142" />
        <q-toggle v-model="draft.podman" label="Podman available (UI / tooling)" class="q-mb-md" />

        <div class="text-subtitle2 q-mb-xs">Host capacity — vCPU: {{ draft.cpu }}</div>
        <q-slider v-model="draft.cpu" :min="8" :max="128" :step="2" label class="q-mb-md" />
        <div class="text-subtitle2 q-mb-xs">RAM: {{ draft.memoryMiB }} MiB ({{ (draft.memoryMiB / 1024).toFixed(0) }} GiB)</div>
        <q-slider v-model="draft.memoryMiB" :min="16384" :max="262144" :step="1024" label class="q-mb-md" />

        <div class="row items-center q-mb-xs">
          <div class="text-subtitle2">Disks ({{ (draft.disks || []).length }}) — {{ diskTotal }} GiB total</div>
          <q-space />
          <q-btn flat dense size="sm" icon="add" label="Disk" @click="addDisk" />
        </div>
        <div v-for="(d, i) in draft.disks || []" :key="'d'+i" class="row q-col-gutter-sm q-mb-sm items-center">
          <div class="col-3"><q-input v-model="d.name" outlined dense label="Name" /></div>
          <div class="col-3"><q-input v-model.number="d.sizeGiB" type="number" outlined dense label="GiB" /></div>
          <div class="col-2"><q-select v-model="d.bus" :options="diskBuses" outlined dense label="Bus" /></div>
          <div class="col-3"><q-select v-model="d.role" :options="diskRoles" outlined dense label="Role" /></div>
          <div class="col-1">
            <q-btn flat dense round icon="delete" size="sm" color="negative"
              :disable="(draft.disks || []).length <= 1" @click="draft.disks.splice(i, 1)" />
          </div>
        </div>

        <div class="row items-center q-mb-xs q-mt-md">
          <div class="text-subtitle2">NICs ({{ (draft.nics || []).length }})</div>
          <q-space />
          <q-btn flat dense size="sm" icon="add" label="NIC" @click="addNic" />
        </div>
        <div v-for="(n, i) in draft.nics || []" :key="'n'+i" class="row q-col-gutter-sm q-mb-sm items-center">
          <div class="col-2"><q-input v-model="n.name" outlined dense label="Name" /></div>
          <div class="col-2"><q-select v-model="n.model" :options="nicModels" outlined dense label="Model" /></div>
          <div class="col-3"><q-select v-model="n.mode" :options="nicModes" outlined dense label="Mode" /></div>
          <div class="col-2"><q-select v-model="n.role" :options="nicRoles" outlined dense label="Role" /></div>
          <div class="col-2"><q-input v-model="n.network" outlined dense label="Net" /></div>
          <div class="col-1">
            <q-btn flat dense round icon="delete" size="sm" color="negative"
              :disable="(draft.nics || []).length <= 1" @click="draft.nics.splice(i, 1)" />
          </div>
        </div>

        <q-input v-model="draft.acmReference" outlined dense type="textarea" autogrow label="ACM CRD reference note" class="q-mb-sm q-mt-md" />
        <q-input v-model="draft.notes" outlined dense type="textarea" autogrow label="Notes" />
      </q-card-section>

      <q-card-section v-else-if="kind === 'gateway'">
        <q-banner dense rounded class="bg-orange-1 text-orange-10 q-mb-md">
          <strong>VYOS-GW</strong> — edge router VM on MACHINE-HOST.
          eth0 = WAN (bridged), eth1 = LAN (libvirt <code>ocp-lab</code> / obscure private CIDR).
          Hub + deployment guests sit on LAN; NAT/FW later (installer TBD).
        </q-banner>
        <q-input v-model="draft.label" outlined dense label="Label" class="q-mb-sm" />
        <q-input v-model="draft.hostname" outlined dense label="Hostname" class="q-mb-sm" />
        <q-input v-model="draft.image" outlined dense label="Image" class="q-mb-sm" hint="vyos" />
        <q-input v-model="draft.isoPath" outlined dense label="ISO path (MVP gap)" class="q-mb-sm" />
        <q-select v-model="draft.phase" :options="gwPhases" outlined dense label="Phase" class="q-mb-sm" />
        <q-input v-model="draft.wanBridge" outlined dense label="WAN bridge" class="q-mb-sm" />
        <q-input v-model="draft.lanNetwork" outlined dense label="LAN libvirt network" class="q-mb-sm" />
        <q-input v-model="draft.lanCIDR" outlined dense label="LAN CIDR" class="q-mb-sm" />
        <q-input v-model="draft.lanIP" outlined dense label="LAN IP (gateway)" class="q-mb-sm" />
        <q-toggle v-model="draft.nat" label="NAT WAN↔LAN" class="q-mb-sm" />
        <q-toggle v-model="draft.firewall" label="Firewall" class="q-mb-md" />
        <div class="text-subtitle2 q-mb-xs">vCPU: {{ draft.cpu }}</div>
        <q-slider v-model="draft.cpu" :min="1" :max="4" :step="1" label class="q-mb-md" />
        <div class="text-subtitle2 q-mb-xs">RAM: {{ draft.memoryMiB }} MiB</div>
        <q-slider v-model="draft.memoryMiB" :min="1024" :max="8192" :step="512" label class="q-mb-md" />
        <div class="text-subtitle2 q-mb-xs">Disk: {{ draft.diskGiB }} GiB</div>
        <q-slider v-model="draft.diskGiB" :min="4" :max="40" :step="1" label class="q-mb-md" />
        <q-input v-model="draft.notes" outlined dense type="textarea" autogrow label="Notes" />
      </q-card-section>

      <q-card-section v-else-if="kind === 'hub'">
        <q-input v-model="draft.label" outlined dense label="Label" class="q-mb-sm" />
        <q-input v-model="draft.hostname" outlined dense label="Hostname" class="q-mb-sm" />
        <q-select v-model="draft.profile" :options="hubProfiles" outlined dense label="Profile"
          class="q-mb-sm" @update:model-value="applyHubProfile" />
        <q-input v-model="draft.version" outlined dense label="OCP version" class="q-mb-sm" />
        <q-input v-model="draft.ip" outlined dense label="IP" class="q-mb-sm" />
        <q-input v-model="draft.mac" outlined dense label="MAC" class="q-mb-md" />

        <div class="text-subtitle2 q-mb-xs">vCPU: {{ draft.cpu }}</div>
        <q-slider v-model="draft.cpu" :min="4" :max="16" :step="1" label class="q-mb-md" />
        <div class="text-subtitle2 q-mb-xs">RAM: {{ draft.memoryMiB }} MiB ({{ (draft.memoryMiB / 1024).toFixed(1) }} GiB)</div>
        <q-slider v-model="draft.memoryMiB" :min="8192" :max="49152" :step="1024" label class="q-mb-md" />
        <div class="text-subtitle2 q-mb-xs">Disk / node: {{ draft.diskGiB }} GiB</div>
        <q-slider v-model="draft.diskGiB" :min="100" :max="400" :step="10" label class="q-mb-sm"
          @update:model-value="syncGuestDisk" />
        <div class="text-caption text-grey-7 q-mb-md">
          Guest shape: 1× virtio disk · 1× NIC (libvirt-network) — extend disks/NICs later
        </div>
        <q-toggle v-model="draft.installACM" label="Install ACM after hub OCP" />
      </q-card-section>

      <q-card-section v-else-if="kind === 'acm'">
        <q-input v-model="draft.label" outlined dense label="Label" class="q-mb-sm" />
        <q-toggle v-model="draft.enabled" label="Enabled" class="q-mb-sm" />
        <q-input v-model="draft.mceChannel" outlined dense label="MCE channel" class="q-mb-sm" />
        <q-input v-model="draft.acmChannel" outlined dense label="ACM channel" class="q-mb-sm" />
        <q-input v-model="draft.notes" outlined dense type="textarea" autogrow label="Notes" />
      </q-card-section>

      <q-card-section v-else-if="kind === 'cluster'">
        <q-input v-model="draft.label" outlined dense label="Label" class="q-mb-sm" />
        <q-input v-model="draft.name" outlined dense label="Cluster name" class="q-mb-sm" />
        <q-select v-model="draft.profile" :options="clusterProfiles" outlined dense label="Profile"
          class="q-mb-sm" @update:model-value="applyClusterProfile" />
        <q-select v-model="draft.phase" :options="phaseOptions" outlined dense label="Lifecycle phase" class="q-mb-sm" />
        <q-input v-model="draft.version" outlined dense label="OCP version" class="q-mb-sm"
          @update:model-value="onVersion" />
        <q-input v-model="draft.clusterImageSet" outlined dense label="ClusterImageSet" class="q-mb-sm" />
        <q-input v-model.number="draft.count" type="number" outlined dense label="Node count" class="q-mb-sm"
          hint="MVP: 3 compact masters" />
        <q-input v-model="draft.ipBase" outlined dense label="IP base (master-0)" class="q-mb-sm" />
        <q-input v-model="draft.macPrefix" outlined dense label="MAC prefix" class="q-mb-sm" />
        <q-input v-model="draft.apiVIP" outlined dense label="API VIP" class="q-mb-sm" />
        <q-input v-model="draft.ingressVIP" outlined dense label="Ingress VIP" class="q-mb-md" />

        <div class="text-subtitle2 q-mb-xs">vCPU / node: {{ draft.cpu }}</div>
        <q-slider v-model="draft.cpu" :min="2" :max="8" :step="1" label class="q-mb-md" />
        <div class="text-subtitle2 q-mb-xs">RAM / node: {{ draft.memoryMiB }} MiB ({{ (draft.memoryMiB / 1024).toFixed(1) }} GiB)</div>
        <q-slider v-model="draft.memoryMiB" :min="8192" :max="32768" :step="1024" label class="q-mb-md" />
        <div class="text-subtitle2 q-mb-xs">Disk / node: {{ draft.diskGiB }} GiB</div>
        <q-slider v-model="draft.diskGiB" :min="80" :max="200" :step="10" label class="q-mb-sm"
          @update:model-value="syncGuestDisk" />
        <div class="text-caption text-grey-7 q-mb-md">
          Per-node: 1× virtio disk · 1× flat NIC on lab net
        </div>
        <q-input v-model="draft.discoveryISO" outlined dense label="Discovery ISO path (optional)" />
      </q-card-section>

      <q-card-actions align="right">
        <q-btn flat label="Cancel" v-close-popup />
        <q-btn color="primary" label="Save" @click="save" />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup>
import { computed, reactive, watch } from 'vue'

const props = defineProps({
  modelValue: Boolean,
  kind: { type: String, default: 'hub' },
  node: { type: Object, default: null },
})
const emit = defineEmits(['update:modelValue', 'save'])

const draft = reactive({})
const hubProfiles = ['hub-supported', 'hub-lab']
const clusterProfiles = ['supported', 'lab-small']
const phaseOptions = ['planned', 'created', 'installing', 'ready', 'destroy']
const infraKinds = ['nested-vm', 'baremetal']
const infraOS = ['rhel-10', 'rhel-9']
const hypervisors = ['vmware', 'kvm', 'none']
const diskBuses = ['nvme', 'virtio', 'sata']
const diskRoles = ['system', 'data', 'pool']
const nicModels = ['virtio', 'e1000e']
const nicModes = ['bridged', 'host-only', 'nat', 'isolated', 'libvirt-network']
const nicRoles = ['uplink', 'host-only', 'wan', 'lan', 'guest']
const gwPhases = ['planned', 'booted', 'configured']

const title = computed(() => {
  if (props.kind === 'infraHost') return 'Edit MACHINE-HOST'
  if (props.kind === 'gateway') return 'Edit VYOS-GW'
  if (props.kind === 'hub') return 'Edit MGMT-CLUSTER'
  if (props.kind === 'acm') return 'Edit ACM'
  return 'Edit DEPLOYMENT-CLUSTER'
})

const diskTotal = computed(() =>
  (draft.disks || []).reduce((n, d) => n + (Number(d.sizeGiB) || 0), 0),
)

watch(
  () => [props.modelValue, props.node],
  () => {
    if (props.modelValue && props.node) {
      Object.keys(draft).forEach((k) => delete draft[k])
      Object.assign(draft, JSON.parse(JSON.stringify(props.node)))
      if (!draft.disks) draft.disks = []
      if (!draft.nics) draft.nics = []
    }
  },
  { immediate: true },
)

function addDisk() {
  if (!draft.disks) draft.disks = []
  draft.disks.push({ name: `disk${draft.disks.length}`, sizeGiB: 100, bus: 'nvme', role: 'data' })
}

function addNic() {
  if (!draft.nics) draft.nics = []
  draft.nics.push({
    name: `eth${draft.nics.length}`, model: 'virtio', mode: 'bridged',
    network: 'bridged-auto', role: 'uplink',
  })
}

function syncGuestDisk(size) {
  draft.diskGiB = size
  if (!draft.disks || !draft.disks.length) {
    draft.disks = [{ name: 'vda', sizeGiB: size, bus: 'virtio', role: 'system' }]
  } else {
    draft.disks[0].sizeGiB = size
  }
}

function applyHubProfile(p) {
  if (p === 'hub-lab') {
    draft.cpu = 8
    draft.memoryMiB = 16384
    draft.diskGiB = 160
  } else {
    draft.cpu = 8
    draft.memoryMiB = 24576
    draft.diskGiB = 200
  }
  syncGuestDisk(draft.diskGiB)
}

function applyClusterProfile(p) {
  if (p === 'lab-small') {
    draft.cpu = 4
    draft.memoryMiB = 12288
    draft.diskGiB = 120
  } else {
    draft.cpu = 4
    draft.memoryMiB = 16384
    draft.diskGiB = 120
  }
  syncGuestDisk(draft.diskGiB)
}

function onVersion(v) {
  const compact = String(v || '').replace(/\./g, '')
  if (compact) draft.clusterImageSet = `img${compact}-x86-64-appsub`
}

function save() {
  if (props.kind === 'infraHost') {
    draft.diskGiB = diskTotal.value
  }
  if (props.kind === 'gateway' && draft.disks?.length) {
    draft.disks[0].sizeGiB = draft.diskGiB
  }
  emit('save', { kind: props.kind, node: { ...draft } })
  emit('update:modelValue', false)
}
</script>
