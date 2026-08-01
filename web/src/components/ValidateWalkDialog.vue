<template>
  <q-dialog :model-value="modelValue" persistent @update:model-value="onDialog">
    <q-card class="validate-dialog">
      <q-card-section class="row items-center q-pb-sm">
        <div>
          <div class="text-h6">Validate walk</div>
          <div class="text-caption text-grey-7">
            {{ title }} — cycling object types and relational neighbours
          </div>
        </div>
        <q-space />
        <q-badge v-if="done" :color="result?.ok ? 'positive' : 'warning'" class="text-capitalize">
          {{ result?.ok ? 'ok' : 'issues' }}
        </q-badge>
        <q-badge v-else color="deep-purple-6">walking…</q-badge>
      </q-card-section>

      <q-separator />

      <q-card-section>
        <!-- Active flash station -->
        <div class="focus" :class="{ flash: !done && current }">
          <q-icon
            :name="current?.icon || 'radar'"
            size="42px"
            :color="focusColor"
            class="focus-icon"
            :class="{ pulse: !done }"
          />
          <div class="focus-text">
            <div class="focus-kind">{{ current?.kind || '…' }}</div>
            <div class="focus-name">{{ current ? `${current.kind}: ${current.name}` : 'Starting walk…' }}</div>
            <div v-if="current?.relates?.length" class="focus-rels">
              <div v-for="(r, i) in current.relates" :key="i">{{ r }}</div>
            </div>
          </div>
        </div>

        <!-- Dynamic icon strip -->
        <div class="strip q-mt-md">
          <div
            v-for="(st, i) in displaySteps"
            :key="st.id + '-' + i"
            class="chip"
            :class="[
              st._ui || st.status,
              { active: i === activeIdx && !done },
            ]"
          >
            <q-icon :name="st.icon || 'circle'" size="18px" />
            <span class="chip-label">{{ st.name }}</span>
          </div>
        </div>

        <q-banner v-if="done && result" dense rounded class="q-mt-md" :class="result.ok ? 'bg-green-1 text-green-10' : 'bg-orange-1 text-orange-10'">
          {{ result.summary }}
        </q-banner>

        <div v-if="done && (result?.issues || []).length" class="issues q-mt-sm">
          <div v-for="(iss, i) in result.issues" :key="i" class="issue-row">
            <q-badge :color="iss.severity === 'error' ? 'negative' : 'warning'" :label="iss.severity" class="q-mr-sm" />
            <span>{{ iss.message }}</span>
          </div>
        </div>
      </q-card-section>

      <q-card-actions align="right" class="q-px-md q-pb-md">
        <q-btn v-if="!done" flat disable label="Walking relations…" />
        <q-btn v-else color="deep-purple-7" unelevated label="Close" v-close-popup />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  title: { type: String, default: 'MockUp' },
  /** Full validate API payload including steps */
  result: { type: Object, default: null },
})
const emit = defineEmits(['update:modelValue', 'closed'])

const activeIdx = ref(-1)
const done = ref(false)
const displaySteps = ref([])
let timer = null

const current = computed(() => {
  if (activeIdx.value < 0) return null
  return displaySteps.value[activeIdx.value] || null
})

const focusColor = computed(() => {
  const st = current.value?._ui || current.value?.status
  if (st === 'error') return 'negative'
  if (st === 'warn') return 'orange-9'
  if (st === 'ok' && done.value) return 'positive'
  return 'deep-purple-6'
})

function stop() {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

function startWalk() {
  stop()
  done.value = false
  activeIdx.value = -1
  const steps = (props.result?.steps || []).map((s) => ({ ...s, _ui: 'pending' }))
  displaySteps.value = steps.length
    ? steps
    : [{ id: 'empty', kind: 'Canvas', name: '(no steps)', icon: 'blur_on', status: 'warn', _ui: 'pending' }]

  let i = 0
  // Brief beat before first flash
  timer = setInterval(() => {
    if (i > 0) {
      const prev = displaySteps.value[i - 1]
      if (prev) prev._ui = prev.status || 'ok'
    }
    if (i >= displaySteps.value.length) {
      stop()
      done.value = true
      activeIdx.value = displaySteps.value.length - 1
      displaySteps.value.forEach((s) => {
        s._ui = s.status || 'ok'
      })
      return
    }
    activeIdx.value = i
    displaySteps.value[i]._ui = 'checking'
    i += 1
  }, 420)
}

function onDialog(v) {
  emit('update:modelValue', v)
  if (!v) {
    stop()
    emit('closed')
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (open && props.result) startWalk()
    if (!open) stop()
  },
)

watch(
  () => props.result,
  (r) => {
    if (props.modelValue && r) startWalk()
  },
)

onBeforeUnmount(stop)
</script>

<style scoped>
.validate-dialog {
  width: min(640px, 96vw);
}
.focus {
  display: flex;
  gap: 0.85rem;
  align-items: flex-start;
  padding: 0.85rem 1rem;
  border-radius: 12px;
  background: #f3e5f5;
  border: 1px solid #ce93d8;
  min-height: 5.5rem;
}
.focus.flash {
  animation: borderPulse 0.42s ease-in-out;
}
.focus-icon.pulse {
  animation: iconPulse 0.42s ease-in-out infinite;
}
.focus-kind {
  font-size: 0.7rem;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  font-weight: 700;
  color: #6a1b9a;
}
.focus-name {
  font-size: 1.05rem;
  font-weight: 750;
  color: #263238;
  margin-top: 0.1rem;
}
.focus-rels {
  margin-top: 0.35rem;
  font-size: 0.78rem;
  color: #607d8b;
  line-height: 1.35;
}
.strip {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  max-height: 9rem;
  overflow: auto;
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.2rem 0.45rem;
  border-radius: 999px;
  border: 1px solid #cfd8dc;
  background: #fff;
  color: #90a4ae;
  font-size: 0.68rem;
  max-width: 100%;
}
.chip-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 9rem;
}
.chip.pending { opacity: 0.55; }
.chip.checking {
  border-color: #8e24aa;
  background: #f3e5f5;
  color: #6a1b9a;
  box-shadow: 0 0 0 2px rgba(142, 36, 170, 0.25);
}
.chip.active.checking {
  transform: scale(1.04);
}
.chip.ok {
  border-color: #a5d6a7;
  background: #e8f5e9;
  color: #2e7d32;
}
.chip.warn {
  border-color: #ffcc80;
  background: #fff3e0;
  color: #ef6c00;
}
.chip.error {
  border-color: #ef9a9a;
  background: #ffebee;
  color: #c62828;
}
.issues {
  max-height: 160px;
  overflow: auto;
}
.issue-row {
  font-size: 0.8rem;
  margin-bottom: 0.35rem;
  line-height: 1.35;
  color: #455a64;
}
@keyframes iconPulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.55; transform: scale(1.12); }
}
@keyframes borderPulse {
  from { box-shadow: 0 0 0 0 rgba(142, 36, 170, 0.35); }
  to { box-shadow: 0 0 0 6px rgba(142, 36, 170, 0); }
}
</style>
