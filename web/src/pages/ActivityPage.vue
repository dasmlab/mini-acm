<template>
  <q-page class="activity-page" padding>
    <header class="page-head">
      <div>
        <p class="page-kicker">Internal</p>
        <h1 class="page-title">Activity</h1>
        <p class="page-lead">Login and navigation events for authenticated users (newest first).</p>
      </div>
      <div class="head-actions">
        <q-select
          v-model="typeFilter"
          dense
          outlined
          emit-value
          map-options
          :options="typeOptions"
          label="Type"
          style="min-width: 140px"
        />
        <q-btn color="primary" outline icon="refresh" label="Refresh" :loading="loading" @click="load" />
      </div>
    </header>

    <div v-if="loading && !events.length" class="row justify-center q-my-xl">
      <q-spinner size="3em" color="primary" />
    </div>

    <div v-else-if="!filtered.length" class="empty-state">
      <q-icon name="history" size="56px" color="blue-grey-5" />
      <div class="empty-title">No events yet</div>
      <div class="empty-sub">Logins and route changes will appear here.</div>
    </div>

    <ul v-else class="event-list">
      <li v-for="(ev, i) in filtered" :key="`${ev.ts}-${i}`" class="event-row">
        <span class="event-ts">{{ formatTs(ev.ts) }}</span>
        <span class="event-body">{{ formatLine(ev) }}</span>
      </li>
    </ul>
  </q-page>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { listActivity } from 'src/services/api'

const events = ref([])
const loading = ref(false)
const typeFilter = ref('all')
const typeOptions = [
  { label: 'All', value: 'all' },
  { label: 'Login', value: 'login' },
  { label: 'Navigate', value: 'navigate' },
  { label: 'Engaged', value: 'engaged' },
]

let poll = null

const filtered = computed(() => {
  if (typeFilter.value === 'all') return events.value
  return events.value.filter((e) => e.type === typeFilter.value)
})

function formatTs(ts) {
  if (!ts) return ''
  try {
    return new Date(ts).toLocaleString()
  } catch {
    return String(ts)
  }
}

function formatDuration(ms) {
  if (ms == null || ms <= 0) return null
  if (ms < 1000) return `${ms}ms`
  const s = ms / 1000
  if (s < 60) return `${s.toFixed(1)}s`
  const m = Math.floor(s / 60)
  const rem = Math.round(s % 60)
  return `${m}m ${rem}s`
}

function formatLine(ev) {
  const who = ev.user || ev.email || ev.sub || 'unknown'
  if (ev.type === 'login') {
    return `${who} logged in`
  }
  const path = ev.path || '/'
  const parts = [`${who} navigated to ${path}`]
  const dwell = formatDuration(ev.dwellMs)
  const visible = formatDuration(ev.visibleMs)
  const engaged = formatDuration(ev.engagedMs)
  const metrics = []
  if (dwell) metrics.push(`dwell ${dwell}`)
  if (visible) metrics.push(`visible ${visible}`)
  if (engaged) metrics.push(`engaged ${engaged}`)
  if (metrics.length) parts.push(`(${metrics.join(', ')})`)
  return parts.join(' ')
}

async function load() {
  loading.value = true
  try {
    const data = await listActivity({ limit: 200 })
    events.value = data.events || []
  } catch (e) {
    events.value = []
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await load()
  poll = setInterval(load, 15_000)
})

onUnmounted(() => {
  if (poll) clearInterval(poll)
})
</script>

<style scoped lang="scss">
.activity-page {
  max-width: 960px;
}
.page-head {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: 1.5rem;
}
.page-kicker {
  margin: 0;
  font-size: 0.75rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: #607d8b;
}
.page-title {
  margin: 0.15rem 0;
  font-size: 1.75rem;
  font-weight: 600;
}
.page-lead {
  margin: 0;
  color: #546e7a;
  max-width: 36rem;
}
.head-actions {
  display: flex;
  gap: 0.75rem;
  align-items: center;
}
.empty-state {
  text-align: center;
  padding: 3rem 1rem;
  color: #78909c;
}
.empty-title {
  font-size: 1.1rem;
  font-weight: 600;
  margin-top: 0.5rem;
  color: #455a64;
}
.empty-sub {
  margin-top: 0.25rem;
}
.event-list {
  list-style: none;
  margin: 0;
  padding: 0;
  border-top: 1px solid #eceff1;
}
.event-row {
  display: grid;
  grid-template-columns: 11rem 1fr;
  gap: 1rem;
  padding: 0.65rem 0;
  border-bottom: 1px solid #eceff1;
  font-size: 0.95rem;
}
.event-ts {
  color: #78909c;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.event-body {
  color: #263238;
  word-break: break-word;
}
@media (max-width: 600px) {
  .event-row {
    grid-template-columns: 1fr;
    gap: 0.2rem;
  }
}
</style>
