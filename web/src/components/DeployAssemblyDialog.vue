<template>
  <q-dialog :model-value="modelValue" persistent @update:model-value="$emit('update:modelValue', $event)">
    <q-card class="deploy-dialog">
      <q-card-section class="row items-center q-pb-sm">
        <div>
          <div class="text-h6">Deploy assembly line</div>
          <div class="text-caption text-grey-7">
            {{ mockupName }} → {{ job?.hostName || 'inventory host' }}
            <span v-if="job?.hostEndpoint">({{ job.hostEndpoint }})</span>
          </div>
        </div>
        <q-space />
        <q-badge :color="jobBadgeColor" class="text-capitalize">{{ job?.status || 'starting' }}</q-badge>
      </q-card-section>

      <q-separator />

      <q-card-section class="q-pt-md">
        <div class="line">
          <div
            v-for="(st, i) in (job?.stages || placeholderStages)"
            :key="st.id"
            class="station"
            :class="st.status"
          >
            <div class="station-icon-wrap">
              <q-spinner v-if="st.status === 'running'" color="orange-9" size="28px" />
              <q-icon v-else :name="stationIcon(st)" size="28px" :color="stationColor(st)" />
            </div>
            <div class="station-label">{{ st.label }}</div>
            <div class="station-detail">{{ st.message || st.detail }}</div>
            <div v-if="i < (job?.stages || placeholderStages).length - 1" class="station-rail" :class="st.status" />
          </div>
        </div>

        <q-banner
          v-if="job?.message"
          dense
          rounded
          class="q-mt-md"
          :class="bannerClass"
        >
          {{ job.message }}
        </q-banner>
      </q-card-section>

      <q-card-actions align="right" class="q-px-md q-pb-md">
        <q-btn
          v-if="job?.status === 'running'"
          flat
          disable
          label="Working on inventory host…"
        />
        <q-btn
          v-else
          color="primary"
          unelevated
          label="Close"
          v-close-popup
        />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { getDeployStatus } from 'src/services/api'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  mockupId: { type: String, default: '' },
  mockupName: { type: String, default: '' },
  /** Initial job from POST /deploy (202) */
  initialJob: { type: Object, default: null },
})
const emit = defineEmits(['update:modelValue', 'finished'])

const job = ref(null)
let timer = null

const placeholderStages = [
  { id: 'generate', label: 'Generate objects', detail: 'ISO/config artifacts', status: 'pending', icon: 'description' },
  { id: 'ee', label: 'Execution environment', detail: 'Podman / Ansible EE', status: 'pending', icon: 'precision_manufacturing' },
  { id: 'vinfra', label: 'Build vInfra', detail: 'libvirt net + pool + vHosts', status: 'pending', icon: 'lan' },
  { id: 'ocp', label: 'Deploy OCP + appliances', detail: 'OCP-MGMT + VyOS', status: 'pending', icon: 'memory' },
  { id: 'acm', label: 'Install ACM', detail: 'Operators on mgmt', status: 'pending', icon: 'extension' },
  { id: 'spokes', label: 'Deployment clusters', detail: 'ACM spokes + ISO', status: 'pending', icon: 'developer_board' },
]

const jobBadgeColor = computed(() => ({
  running: 'orange-8',
  succeeded: 'positive',
  failed: 'negative',
}[job.value?.status] || 'grey-6'))

const bannerClass = computed(() => {
  if (job.value?.status === 'succeeded') return 'bg-green-1 text-green-10'
  if (job.value?.status === 'failed') return 'bg-orange-1 text-orange-10'
  return 'bg-blue-1 text-blue-10'
})

function stationIcon(st) {
  if (st.status === 'ok') return 'check_circle'
  if (st.status === 'failed') return 'error'
  if (st.status === 'blocked') return 'pause_circle'
  return st.icon || 'radio_button_unchecked'
}

function stationColor(st) {
  return {
    ok: 'positive',
    failed: 'negative',
    blocked: 'orange-9',
    running: 'orange-9',
    pending: 'grey-5',
  }[st.status] || 'grey-5'
}

async function poll() {
  if (!props.mockupId) return
  try {
    const data = await getDeployStatus(props.mockupId)
    job.value = data.job
    if (data.job?.status && data.job.status !== 'running') {
      stopPoll()
      emit('finished', data)
    }
  } catch {
    /* keep last job snapshot */
  }
}

function startPoll() {
  stopPoll()
  timer = setInterval(poll, 1500)
  poll()
}

function stopPoll() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      job.value = props.initialJob
      startPoll()
    } else {
      stopPoll()
    }
  },
)

watch(
  () => props.initialJob,
  (j) => {
    if (j) job.value = j
  },
)

onBeforeUnmount(stopPoll)
</script>

<style scoped>
.deploy-dialog {
  width: min(720px, 96vw);
}
.line {
  display: grid;
  grid-template-columns: repeat(6, 1fr);
  gap: 0.35rem;
  position: relative;
}
.station {
  text-align: center;
  position: relative;
  padding: 0.25rem 0.2rem 0.5rem;
}
.station-icon-wrap {
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.station-label {
  font-size: 0.72rem;
  font-weight: 700;
  color: #263238;
  line-height: 1.2;
  margin-top: 0.25rem;
  min-height: 2.2em;
}
.station-detail {
  font-size: 0.65rem;
  color: #78909c;
  line-height: 1.25;
  min-height: 2.4em;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
.station.running .station-label { color: #ef6c00; }
.station.ok .station-label { color: #2e7d32; }
.station.failed .station-label,
.station.blocked .station-label { color: #c62828; }
.station-rail {
  display: none;
}
@media (max-width: 700px) {
  .line {
    grid-template-columns: 1fr;
  }
  .station {
    text-align: left;
    display: grid;
    grid-template-columns: 40px 1fr;
    grid-template-rows: auto auto;
    column-gap: 0.6rem;
  }
  .station-icon-wrap { grid-row: 1 / span 2; }
  .station-label { min-height: 0; margin-top: 0; }
  .station-detail { min-height: 0; -webkit-line-clamp: 4; }
}
</style>
