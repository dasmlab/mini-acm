<template>
  <q-dialog :model-value="modelValue" @update:model-value="onDialog">
    <q-card class="costme-dialog">
      <q-card-section class="row items-center q-pb-sm">
        <div>
          <div class="text-h6">COST-ME</div>
          <div class="text-caption text-grey-7">
            {{ title }} — estimate via cheapcloud (no live Azure create)
          </div>
        </div>
        <q-space />
        <q-btn flat round dense icon="close" v-close-popup />
      </q-card-section>
      <q-separator />
      <q-card-section v-if="loading" class="text-center q-pa-lg">
        <q-spinner color="teal" size="40px" />
        <div class="q-mt-sm text-caption">Asking cheapcloud…</div>
      </q-card-section>
      <q-card-section v-else-if="error" class="text-negative">
        {{ error }}
        <div v-if="url" class="text-caption text-grey-7 q-mt-sm">URL: {{ url }}</div>
      </q-card-section>
      <q-card-section v-else-if="report" class="q-gutter-md">
        <div class="row q-col-gutter-sm">
          <div class="col-6">
            <div class="metric-label">Est hourly</div>
            <div class="metric-val">${{ num(report.total_est_hourly_usd) }}/hr</div>
          </div>
          <div class="col-6">
            <div class="metric-label">Est monthly</div>
            <div class="metric-val">${{ num(report.total_est_monthly_usd) }}/mo</div>
          </div>
        </div>

        <div>
          <div class="section-title">Lines</div>
          <q-markup-table flat dense bordered>
            <thead>
              <tr>
                <th>Capability</th>
                <th>Provider</th>
                <th>×N</th>
                <th>$/hr</th>
                <th>$/mo</th>
                <th>Conf</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(line, i) in (report.lines || [])" :key="i">
                <td>{{ line.capability }}</td>
                <td>{{ line.provider }}{{ line.sku ? ' · ' + line.sku : '' }}</td>
                <td>{{ line.count || 1 }}</td>
                <td>${{ num(line.estimate?.hourlyUsd) }}</td>
                <td>${{ num(line.estimate?.usd) }}</td>
                <td>{{ line.estimate?.confidence || '—' }}</td>
              </tr>
            </tbody>
          </q-markup-table>
        </div>

        <div v-if="(report.envelopes || []).length">
          <div class="section-title">Envelopes</div>
          <div v-for="(e, i) in report.envelopes" :key="i" class="envelope-row">
            <q-badge :color="envColor(e.status)" class="q-mr-sm">{{ e.status }}</q-badge>
            <strong>{{ e.name }}</strong>
            · max ${{ num(e.monthly_max_usd) }}
            <span v-if="e.free_tier_pct != null"> · free-tier {{ num(e.free_tier_pct) }}%</span>
            <div class="text-caption text-grey-7">{{ e.notes }}</div>
          </div>
        </div>

        <div v-if="(report.alternatives || []).length">
          <div class="section-title">Cheaper alternatives</div>
          <div v-for="(a, i) in report.alternatives" :key="i" class="text-body2 q-mb-xs">
            {{ a.for_capability }} → <strong>{{ a.provider }}</strong>
            (~${{ num(a.estimate?.usd) }}/mo) — {{ a.reason }}
          </div>
        </div>

        <div v-if="(report.notes || []).length" class="text-caption text-grey-7">
          <div v-for="(n, i) in report.notes" :key="i">• {{ n }}</div>
        </div>
      </q-card-section>
      <q-card-actions align="right">
        <q-btn flat label="Close" v-close-popup />
      </q-card-actions>
    </q-card>
  </q-dialog>
</template>

<script setup>
const props = defineProps({
  modelValue: { type: Boolean, default: false },
  title: { type: String, default: 'MockUp' },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
  url: { type: String, default: '' },
  report: { type: Object, default: null },
})
const emit = defineEmits(['update:modelValue'])

function onDialog(v) { emit('update:modelValue', v) }
function num(v) {
  if (v == null || v === '') return '—'
  const n = Number(v)
  if (Number.isNaN(n)) return '—'
  return n.toFixed(4)
}
function envColor(s) {
  if (s === 'ok') return 'positive'
  if (s === 'warn') return 'warning'
  if (s === 'breach') return 'negative'
  return 'grey'
}
</script>

<style scoped>
.costme-dialog { min-width: min(640px, 92vw); max-width: 720px; }
.metric-label { font-size: 11px; text-transform: uppercase; color: #6f7f8d; letter-spacing: .06em; }
.metric-val { font-size: 22px; font-weight: 600; color: #2f8f7d; }
.section-title { font-size: 11px; text-transform: uppercase; color: #6f7f8d; letter-spacing: .08em; margin-bottom: 6px; }
.envelope-row { margin-bottom: 8px; font-size: 13px; }
</style>
