<template>
  <q-dialog :model-value="modelValue" persistent @update:model-value="$emit('update:modelValue', $event)">
    <q-card class="create-dialog">
      <q-card-section class="row items-center q-pb-none">
        <div class="text-h6">New MockUp</div>
        <q-space />
        <q-btn flat dense round icon="close" v-close-popup />
      </q-card-section>

      <q-card-section>
        <p class="text-body2 text-grey-7 q-mb-md">
          Pick a genre and style, name it, then choose how to configure.
        </p>

        <q-select
          v-model="form.genre"
          :options="genreOptions"
          emit-value
          map-options
          outlined
          dense
          label="Genre"
          class="q-mb-md"
          @update:model-value="onGenreChange"
        />
        <q-select
          v-model="form.style"
          :options="styleOptions"
          emit-value
          map-options
          outlined
          dense
          label="Type / style"
          class="q-mb-md"
          :hint="styleHint"
        />
        <q-input
          v-model="form.name"
          outlined
          dense
          label="Name"
          class="q-mb-md"
          hint="e.g. lab-rack-1"
        />

        <div class="text-subtitle2 q-mb-sm">How do you want to configure?</div>
        <div class="path-grid">
          <button
            type="button"
            class="path-card"
            :class="{ active: form.path === 'defaults' }"
            @click="form.path = 'defaults'"
          >
            <q-icon name="bolt" size="22px" color="orange-9" />
            <span class="path-title">Use defaults</span>
            <span class="path-desc">Fast click-through — seed rack, derive YAML, then Validate → Deploy.</span>
          </button>
          <button
            type="button"
            class="path-card"
            :class="{ active: form.path === 'wizard' }"
            @click="form.path = 'wizard'"
          >
            <q-icon name="playlist_play" size="22px" color="primary" />
            <span class="path-title">Wizard</span>
            <span class="path-desc">Collect gaps step-by-step (pull secret, SSH key, ISO paths) then derive.</span>
          </button>
          <button
            type="button"
            class="path-card"
            :class="{ active: form.path === 'manual' }"
            @click="form.path = 'manual'"
          >
            <q-icon name="account_tree" size="22px" color="teal-8" />
            <span class="path-title">Manual</span>
            <span class="path-desc">Open Topology and edit the canvas / objects yourself.</span>
          </button>
        </div>
      </q-card-section>

      <q-card-actions align="right" class="q-px-md q-pb-md">
        <q-btn flat label="Cancel" v-close-popup />
        <q-btn
          color="primary"
          unelevated
          :label="createLabel"
          :loading="creating"
          :disable="!canCreate"
          @click="submit"
        />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { Notify } from 'quasar'
import { createMockup, deriveMockup } from 'src/services/api'

const props = defineProps({
  modelValue: { type: Boolean, default: false },
  catalog: { type: Object, default: () => ({ genres: [], styles: [] }) },
  /** Prefill style when opened from a carousel slide */
  preferStyle: { type: String, default: '' },
  preferGenre: { type: String, default: '' },
  preferPath: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue', 'created'])

const router = useRouter()
const creating = ref(false)
const form = reactive({
  genre: 'cluster-management',
  style: 'acm-multi-cluster',
  name: 'lab-rack-1',
  baseDomain: 'lab.example.net',
  provider: 'libvirt',
  notes: '',
  path: 'defaults',
})

const genreOptions = computed(() =>
  (props.catalog.genres || []).map((g) => ({
    label: g.label,
    value: g.id,
  })),
)

const styleOptions = computed(() => {
  const styles = (props.catalog.styles || []).filter((s) => s.genre === form.genre)
  return styles.map((s) => ({
    label: s.available ? s.label : `${s.label} (soon)`,
    value: s.id,
    disable: !s.available,
    description: s.description,
  }))
})

const selectedStyle = computed(() =>
  (props.catalog.styles || []).find((s) => s.id === form.style),
)

const styleHint = computed(() => selectedStyle.value?.description || '')
const canCreate = computed(() => !!form.name?.trim() && !!selectedStyle.value?.available)
const createLabel = computed(() => {
  if (form.path === 'wizard') return 'Create & open Wizard'
  if (form.path === 'manual') return 'Create & open Topology'
  return 'Create with defaults'
})

function onGenreChange() {
  const first = styleOptions.value.find((o) => !o.disable) || styleOptions.value[0]
  form.style = first?.value || ''
}

function resetForm() {
  form.genre = props.preferGenre || 'cluster-management'
  form.style = props.preferStyle || 'acm-multi-cluster'
  form.name = 'lab-rack-1'
  form.baseDomain = 'lab.example.net'
  form.provider = 'libvirt'
  form.notes = ''
  form.path = ['defaults', 'wizard', 'manual'].includes(props.preferPath)
    ? props.preferPath
    : 'defaults'
  if (!styleOptions.value.some((o) => o.value === form.style && !o.disable)) {
    onGenreChange()
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) resetForm()
  },
)

async function submit() {
  if (!canCreate.value) {
    Notify.create({ type: 'warning', message: 'Choose an available style and a name.' })
    return
  }
  creating.value = true
  try {
    const payload = {
      genre: form.genre,
      style: form.style,
      name: form.name.trim(),
      notes: form.notes,
    }
    if (form.style === 'acm-multi-cluster' || form.style === 'single-sno-ocp') {
      payload.baseDomain = form.baseDomain
      payload.provider = form.provider
    }
    const m = await createMockup(payload)
    const id = m.metadata?.id
    emit('update:modelValue', false)
    emit('created', { mockup: m, path: form.path })

    if (form.path === 'defaults' && id) {
      try {
        await deriveMockup(id)
        Notify.create({
          type: 'positive',
          message: `${form.name} ready with defaults (Configured) — Validate then Deploy when inventory is green.`,
          timeout: 6000,
        })
      } catch (e) {
        Notify.create({ type: 'warning', message: `Created, but derive failed: ${e.response?.data || e.message}` })
      }
      await router.push({ name: 'mockups' })
    } else if (form.path === 'wizard' && id) {
      Notify.create({ type: 'positive', message: 'MockUp created — fill gaps in the Wizard.' })
      await router.push({ name: 'wizard', params: { id } })
    } else if (form.path === 'manual' && id) {
      Notify.create({ type: 'positive', message: 'MockUp created — edit Topology manually.' })
      await router.push({ name: 'topology', params: { id } })
    }
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    creating.value = false
  }
}
</script>

<style scoped>
.create-dialog {
  width: min(560px, 96vw);
}
.path-grid {
  display: grid;
  gap: 0.6rem;
}
.path-card {
  display: grid;
  grid-template-columns: 28px 1fr;
  grid-template-rows: auto auto;
  column-gap: 0.65rem;
  row-gap: 0.15rem;
  text-align: left;
  border: 1px solid #cfd8dc;
  border-radius: 10px;
  background: #fff;
  padding: 0.75rem 0.85rem;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s, box-shadow 0.15s;
}
.path-card .q-icon {
  grid-row: 1 / span 2;
  align-self: center;
}
.path-card:hover {
  border-color: #90caf9;
  background: #f7fbff;
}
.path-card.active {
  border-color: #1565c0;
  background: #e3f2fd;
  box-shadow: inset 0 0 0 1px #1565c0;
}
.path-title {
  font-weight: 700;
  color: #263238;
  font-size: 0.92rem;
}
.path-desc {
  color: #607d8b;
  font-size: 0.78rem;
  line-height: 1.35;
}
</style>
