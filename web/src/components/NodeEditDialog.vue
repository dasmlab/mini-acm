<template>
  <q-dialog :model-value="modelValue" @update:model-value="$emit('update:modelValue', $event)" persistent>
    <q-card style="min-width: 480px; max-width: 560px">
      <q-card-section class="row items-center">
        <div class="text-h6">{{ title }}</div>
        <q-space />
        <q-btn flat round dense icon="close" v-close-popup />
      </q-card-section>

      <q-card-section v-if="kind === 'hub'">
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
        <div class="text-subtitle2 q-mb-xs">Disk: {{ draft.diskGiB }} GiB</div>
        <q-slider v-model="draft.diskGiB" :min="100" :max="400" :step="10" label class="q-mb-md" />
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
        <q-input v-model="draft.version" outlined dense label="OCP version" class="q-mb-sm" />
        <q-input v-model.number="draft.count" type="number" outlined dense label="Node count" class="q-mb-sm"
          hint="MVP: 3 compact masters" />
        <q-input v-model="draft.ipBase" outlined dense label="IP base (master-0)" class="q-mb-sm" />
        <q-input v-model="draft.macPrefix" outlined dense label="MAC prefix" class="q-mb-md" />

        <div class="text-subtitle2 q-mb-xs">vCPU / node: {{ draft.cpu }}</div>
        <q-slider v-model="draft.cpu" :min="2" :max="8" :step="1" label class="q-mb-md" />
        <div class="text-subtitle2 q-mb-xs">RAM / node: {{ draft.memoryMiB }} MiB</div>
        <q-slider v-model="draft.memoryMiB" :min="8192" :max="32768" :step="1024" label class="q-mb-md" />
        <div class="text-subtitle2 q-mb-xs">Disk / node: {{ draft.diskGiB }} GiB</div>
        <q-slider v-model="draft.diskGiB" :min="80" :max="200" :step="10" label />
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

const title = computed(() => {
  if (props.kind === 'hub') return 'Edit MGMT-CLUSTER'
  if (props.kind === 'acm') return 'Edit ACM'
  return 'Edit DEPLOYMENT-CLUSTER'
})

watch(
  () => [props.modelValue, props.node],
  () => {
    if (props.modelValue && props.node) {
      Object.keys(draft).forEach((k) => delete draft[k])
      Object.assign(draft, JSON.parse(JSON.stringify(props.node)))
    }
  },
  { immediate: true },
)

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
}

function save() {
  emit('save', { kind: props.kind, node: { ...draft } })
  emit('update:modelValue', false)
}
</script>
