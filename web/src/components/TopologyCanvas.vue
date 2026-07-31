<template>
  <div class="topo-wrap">
    <svg
      class="topo-svg"
      :viewBox="`0 0 ${vbW} ${vbH}`"
      preserveAspectRatio="xMidYMid meet"
      @mousemove="onMove"
      @mouseup="onUp"
      @mouseleave="onUp"
    >
      <defs>
        <pattern id="grid" width="28" height="28" patternUnits="userSpaceOnUse">
          <path d="M 28 0 L 0 0 0 28" fill="none" stroke="rgba(15,40,80,0.06)" stroke-width="1" />
        </pattern>
        <marker id="arrow" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
          <path d="M0,0 L6,3 L0,6 Z" fill="#78909c" />
        </marker>
        <filter id="softShadow" x="-40%" y="-40%" width="180%" height="180%">
          <feDropShadow dx="0" dy="3" stdDeviation="4" flood-color="#0b1f3a" flood-opacity="0.18" />
        </filter>
      </defs>

      <rect width="100%" height="100%" fill="url(#grid)" />

      <!-- edges: hub -> acm, acm -> clusters -->
      <path
        v-for="e in edges"
        :key="e.id"
        :d="e.d"
        class="link"
        fill="none"
        stroke="#78909c"
        stroke-width="2"
        marker-end="url(#arrow)"
      />

      <g
        v-for="n in nodes"
        :key="n.id"
        class="topo-node"
        :class="{ active: selectedId === n.id }"
        filter="url(#softShadow)"
        @mousedown.prevent="onDown($event, n)"
        @click.stop="onClick(n)"
      >
        <rect
          :x="n.x - n.w / 2"
          :y="n.y - 28"
          :width="n.w"
          :height="56"
          rx="12"
          :class="n.cls"
          :stroke="selectedId === n.id ? '#ff6f00' : '#fff'"
          stroke-width="2"
        />
        <text :x="n.x" :y="n.y - 4" text-anchor="middle" class="node-title">{{ n.label }}</text>
        <text :x="n.x" :y="n.y + 14" text-anchor="middle" class="node-sub">{{ n.sub }}</text>
      </g>
    </svg>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'

const props = defineProps({
  mockup: { type: Object, required: true },
  selectedId: { type: String, default: null },
})
const emit = defineEmits(['select', 'move'])

const vbW = 720
const vbH = 480
const dragging = ref(null)
const moved = ref(false)

const pos = (id, fallback) => {
  const p = props.mockup?.layout?.nodes?.[id]
  return p ? { x: p.x, y: p.y } : fallback
}

const nodes = computed(() => {
  const m = props.mockup
  if (!m?.spec) return []
  const out = []
  const hub = m.spec.hub
  const hp = pos(hub.id, { x: 320, y: 280 })
  out.push({
    id: hub.id,
    kind: 'hub',
    label: hub.label || 'MGMT-CLUSTER',
    sub: `${hub.cpu}c / ${Math.round(hub.memoryMiB / 1024)}G`,
    x: hp.x, y: hp.y, w: 160, cls: 'fill-hub',
  })
  const acm = m.spec.acm
  const ap = pos(acm.id, { x: 320, y: 140 })
  out.push({
    id: acm.id,
    kind: 'acm',
    label: acm.label || 'ACM',
    sub: acm.enabled ? 'enabled' : 'disabled',
    x: ap.x, y: ap.y, w: 120, cls: 'fill-acm',
  })
  ;(m.spec.clusters || []).forEach((c, i) => {
    const cp = pos(c.id, { x: 520, y: 160 + i * 100 })
    out.push({
      id: c.id,
      kind: 'cluster',
      label: c.label || c.name,
      sub: `${c.count}× ${c.cpu}c/${Math.round(c.memoryMiB / 1024)}G`,
      x: cp.x, y: cp.y, w: 180, cls: 'fill-cluster',
      clusterIndex: i,
    })
  })
  return out
})

const edges = computed(() => {
  const byId = Object.fromEntries(nodes.value.map((n) => [n.id, n]))
  const hub = byId[props.mockup?.spec?.hub?.id]
  const acm = byId[props.mockup?.spec?.acm?.id]
  const out = []
  if (hub && acm) {
    out.push({ id: 'hub-acm', d: curve(hub.x, hub.y - 28, acm.x, acm.y + 28) })
  }
  ;(props.mockup?.spec?.clusters || []).forEach((c) => {
    const n = byId[c.id]
    if (acm && n) {
      out.push({ id: `acm-${c.id}`, d: curve(acm.x + 60, acm.y, n.x - 90, n.y) })
    }
  })
  return out
})

function curve(x1, y1, x2, y2) {
  const mx = (x1 + x2) / 2
  return `M ${x1} ${y1} Q ${mx} ${y1} ${x2} ${y2}`
}

function onDown(ev, n) {
  dragging.value = { id: n.id, ox: ev.clientX, oy: ev.clientY, sx: n.x, sy: n.y }
  moved.value = false
}

function onMove(ev) {
  if (!dragging.value) return
  const dx = (ev.clientX - dragging.value.ox) * (vbW / (ev.currentTarget.clientWidth || vbW))
  const dy = (ev.clientY - dragging.value.oy) * (vbH / (ev.currentTarget.clientHeight || vbH))
  if (Math.abs(dx) + Math.abs(dy) > 3) moved.value = true
  emit('move', {
    id: dragging.value.id,
    x: Math.max(60, Math.min(vbW - 60, dragging.value.sx + dx)),
    y: Math.max(40, Math.min(vbH - 40, dragging.value.sy + dy)),
  })
}

function onUp() {
  dragging.value = null
}

function onClick(n) {
  if (moved.value) return
  emit('select', n)
}
</script>

<style scoped>
.topo-wrap {
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  background: #fafbfd;
  overflow: hidden;
}
.topo-svg {
  width: 100%;
  height: 480px;
  display: block;
  cursor: grab;
}
.topo-node {
  cursor: pointer;
}
.topo-node.active rect {
  filter: brightness(1.05);
}
.fill-hub { fill: #1a237e; }
.fill-acm { fill: #00838f; }
.fill-cluster { fill: #1565c0; }
.node-title {
  fill: #fff;
  font-size: 12px;
  font-weight: 700;
  pointer-events: none;
}
.node-sub {
  fill: rgba(255, 255, 255, 0.85);
  font-size: 10px;
  pointer-events: none;
}
.link {
  pointer-events: none;
}
</style>
