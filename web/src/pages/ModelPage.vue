<template>
  <q-page class="model-page" padding>
    <header class="page-head">
      <div>
        <p class="page-kicker">Design bench · price &amp; track</p>
        <h1 class="page-title">Model</h1>
        <p class="page-lead">
          Compose multi-cloud blocks here (Spot VM, OCP SNO slim, R2). Modeling stays in mock-me;
          <strong>cheapcloud</strong> prices and tracks after Import.
        </p>
      </div>
      <div class="page-head-actions">
        <q-btn color="teal-8" unelevated icon="add" label="New cloud model" :loading="creating" @click="onCreate" />
        <q-btn outline color="primary" icon="refresh" label="Refresh" :loading="loading" @click="load" />
        <q-btn
          flat
          color="primary"
          icon="open_in_new"
          label="cheapcloud Home"
          href="https://cheapcloud-dasmlab.apps.2026-prod-1.ocp.dasmlab.org/ui"
          target="_blank"
        />
      </div>
    </header>

    <div class="row q-col-gutter-md">
      <div class="col-12 col-md-4">
        <q-card flat bordered class="palette-card">
          <q-card-section>
            <div class="text-subtitle2">Palette (Design bench)</div>
            <div class="text-caption text-grey-7 q-mb-sm">Drop these on a cloud model Topology (free-form).</div>
            <div v-for="g in catalogGroups" :key="g.id" class="q-mb-md">
              <div class="text-caption text-uppercase text-grey-6">{{ g.label }}</div>
              <q-chip
                v-for="it in itemsFor(g.id)"
                :key="it.id"
                dense
                outline
                color="teal-8"
                class="q-ma-xs"
                :icon="it.icon"
                :label="it.label"
              />
            </div>
            <div class="text-caption text-grey-7">{{ catalogNotes }}</div>
          </q-card-section>
        </q-card>
      </div>

      <div class="col-12 col-md-8">
        <q-banner v-if="error" class="bg-orange-2 text-orange-10 q-mb-md" rounded>
          {{ error }}
        </q-banner>
        <div v-if="loading && models.length === 0" class="row justify-center q-my-xl">
          <q-spinner color="teal" size="3em" />
        </div>
        <div v-else-if="models.length === 0" class="empty-state">
          <q-icon name="architecture" size="56px" color="teal-6" />
          <div class="empty-title">No cloud models yet</div>
          <div class="empty-sub">Create a Cloud cost model MockUp, edit Topology, then Cost me / Import &amp; track.</div>
          <q-btn color="teal-8" unelevated icon="add" label="New cloud model" class="q-mt-md" @click="onCreate" />
        </div>
        <div v-else class="model-grid">
          <article v-for="m in models" :key="m.metadata.id" class="model-card">
            <div class="card-body">
              <div class="row items-center no-wrap">
                <h2 class="card-name">{{ m.metadata.name }}</h2>
                <q-space />
                <q-badge v-if="m.status?.cheapcloudProductId" color="teal" outline>
                  tracked · {{ m.status.cheapcloudProductId }}
                </q-badge>
                <q-badge v-else color="grey-6" outline>not imported</q-badge>
              </div>
              <div class="text-caption text-grey-7 q-mt-xs">
                {{ (m.spec?.canvas?.orphans || []).length }} cloud blocks · {{ m.spec?.style }}
              </div>
              <div class="q-mt-md row q-gutter-sm">
                <q-btn
                  dense unelevated color="teal-8" icon="account_tree" label="Open Topology"
                  :to="{ name: 'topology', params: { id: m.metadata.id } }"
                />
                <q-btn dense outline color="primary" icon="payments" label="Cost me" :loading="busyId === m.metadata.id" @click="onCost(m)" />
                <q-btn dense outline color="teal-9" icon="cloud_upload" label="Import & track" :loading="busyId === m.metadata.id + '-i'" @click="onImport(m)" />
              </div>
            </div>
          </article>
        </div>
      </div>
    </div>
  </q-page>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Notify } from 'quasar'
import {
  listMockups, createMockup, costMeMockup, importMockupCheapcloud, getModelCatalog,
} from 'src/services/api'

const router = useRouter()
const loading = ref(false)
const creating = ref(false)
const error = ref('')
const mockups = ref([])
const catalog = ref({ groups: [], items: [], notes: '' })
const busyId = ref('')

const models = computed(() =>
  mockups.value.filter((m) => m.spec?.style === 'cloud-cost-model' || m.spec?.genre === 'infrastructure'),
)

const catalogGroups = computed(() => (catalog.value.groups || []).slice().sort((a, b) => (a.order || 0) - (b.order || 0)))
const catalogNotes = computed(() => catalog.value.notes || '')
function itemsFor(gid) {
  return (catalog.value.items || []).filter((i) => i.group === gid)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [list, cat] = await Promise.all([listMockups(), getModelCatalog().catch(() => ({ groups: [], items: [] }))])
    mockups.value = Array.isArray(list) ? list : (list?.mockups || [])
    catalog.value = cat
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

async function onCreate() {
  creating.value = true
  try {
    const name = `cloud-${new Date().toISOString().slice(0, 10)}`
    const m = await createMockup({
      name,
      genre: 'infrastructure',
      style: 'cloud-cost-model',
      provider: 'multi-cloud',
      notes: 'Design bench model',
    })
    Notify.create({ type: 'positive', message: `Created ${m.metadata.name}` })
    await router.push({ name: 'topology', params: { id: m.metadata.id } })
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data || e.message })
  } finally {
    creating.value = false
  }
}

async function onCost(m) {
  busyId.value = m.metadata.id
  try {
    const res = await costMeMockup(m.metadata.id)
    const mo = res.report?.total_est_monthly_usd
    Notify.create({ type: 'positive', message: `COST-ME ~$${Number(mo || 0).toFixed(2)}/mo` })
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data?.error || e.message })
  } finally {
    busyId.value = ''
  }
}

async function onImport(m) {
  busyId.value = m.metadata.id + '-i'
  try {
    const res = await importMockupCheapcloud(m.metadata.id)
    Notify.create({ type: 'positive', message: `Tracked as ${res.product_id}` })
    await load()
  } catch (e) {
    Notify.create({ type: 'negative', message: e.response?.data?.error || e.message })
  } finally {
    busyId.value = ''
  }
}

onMounted(load)
</script>

<style scoped>
.page-head { display: flex; flex-wrap: wrap; gap: 16px; justify-content: space-between; margin-bottom: 20px; }
.page-kicker { margin: 0; text-transform: uppercase; letter-spacing: .08em; font-size: 11px; color: #5c6e6a; }
.page-title { margin: 4px 0; font-size: 1.75rem; color: #14201e; }
.page-lead { margin: 0; max-width: 40rem; color: #5c6e6a; line-height: 1.45; }
.page-head-actions { display: flex; flex-wrap: wrap; gap: 8px; align-items: flex-start; }
.palette-card { border-radius: 12px; }
.model-grid { display: grid; gap: 12px; }
.model-card { background: #fff; border: 1px solid rgba(20,32,30,.12); border-radius: 12px; overflow: hidden; }
.card-body { padding: 16px; }
.card-name { margin: 0; font-size: 1.1rem; }
.empty-state { text-align: center; padding: 48px 16px; color: #5c6e6a; }
.empty-title { font-size: 1.2rem; font-weight: 600; color: #14201e; margin-top: 8px; }
</style>
