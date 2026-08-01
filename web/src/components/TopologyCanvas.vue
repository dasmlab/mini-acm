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
        <marker id="arrow-host" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
          <path d="M0,0 L6,3 L0,6 Z" fill="#546e7a" />
        </marker>
        <filter id="softShadow" x="-40%" y="-40%" width="180%" height="180%">
          <feDropShadow dx="0" dy="3" stdDeviation="4" flood-color="#0b1f3a" flood-opacity="0.18" />
        </filter>
      </defs>

      <rect width="100%" height="100%" fill="url(#grid)" />

      <path
        v-for="e in edges"
        :key="e.id"
        :d="e.d"
        class="link"
        fill="none"
        :stroke="e.stroke"
        :stroke-width="e.width"
        :stroke-dasharray="e.dash"
        :marker-end="e.marker"
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
          :y="n.y - n.h / 2"
          :width="n.w"
          :height="n.h"
          :rx="n.rx"
          :class="n.cls"
          :stroke="selectedId === n.id ? '#ff6f00' : '#fff'"
          stroke-width="2"
        />
        <text :x="n.x" :y="n.y - 8" text-anchor="middle" class="node-title">{{ n.label }}</text>
        <text :x="n.x" :y="n.y + 12" text-anchor="middle" class="node-sub">{{ n.sub }}</text>
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

const vbW = 780
const vbH = 580
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

  const infra = m.spec.infraHost
  if (infra?.id) {
    const ip = pos(infra.id, { x: 120, y: 500 })
    out.push({
      id: infra.id,
      kind: 'infraHost',
      label: infra.label || 'MACHINE-HOST',
      sub: 'host · libvirt',
      x: ip.x, y: ip.y, w: 210, h: 64, rx: 8, cls: 'fill-infra',
    })
  }

  const gw = m.spec.gateway
  if (gw?.id) {
    const gp = pos(gw.id, { x: 140, y: 340 })
    out.push({
      id: gw.id,
      kind: 'gateway',
      label: gw.label || 'VYOS-GW',
      sub: `edge · ${gw.lanCIDR || 'LAN'}`,
      x: gp.x, y: gp.y, w: 160, h: 60, rx: 10, cls: 'fill-gateway',
    })
  }

  const hub = m.spec.hub
  const hp = pos(hub.id, { x: 340, y: 300 })
  out.push({
    id: hub.id,
    kind: 'hub',
    label: hub.label || 'MGMT-CLUSTER',
    sub: `runs ACM · ${hub.cpu}c/${Math.round(hub.memoryMiB / 1024)}G`,
    x: hp.x, y: hp.y, w: 188, h: 60, rx: 12, cls: 'fill-hub',
  })

  const acm = m.spec.acm
  const ap = pos(acm.id, { x: 340, y: 150 })
  out.push({
    id: acm.id,
    kind: 'acm',
    label: acm.label || 'ACM',
    sub: acm.enabled ? 'governs' : 'off',
    x: ap.x, y: ap.y, w: 120, h: 60, rx: 12, cls: 'fill-acm',
  })

  ;(m.spec.clusters || []).forEach((c, i) => {
    const cp = pos(c.id, { x: 580, y: 160 + i * 120 })
    out.push({
      id: c.id,
      kind: 'cluster',
      label: c.label || c.name,
      sub: `managed · ${c.count}×${c.cpu}c · ${c.phase || 'planned'}`,
      x: cp.x, y: cp.y, w: 210, h: 60, rx: 12, cls: 'fill-cluster',
      clusterIndex: i,
    })
  })
  return out
})

const edges = computed(() => {
  const byId = Object.fromEntries(nodes.value.map((n) => [n.id, n]))
  const infra = byId[props.mockup?.spec?.infraHost?.id]
  const gw = byId[props.mockup?.spec?.gateway?.id]
  const hub = byId[props.mockup?.spec?.hub?.id]
  const acm = byId[props.mockup?.spec?.acm?.id]
  const out = []

  // MACHINE-HOST runs VyOS + guest VMs
  if (infra && gw) {
    out.push({
      id: 'infra-gw',
      d: curve(infra.x, infra.y - 32, gw.x, gw.y + 30),
      stroke: '#546e7a', width: 2, dash: '6 4', marker: 'url(#arrow-host)',
    })
  }
  if (infra && hub) {
    out.push({
      id: 'infra-hub',
      d: curve(infra.x + 70, infra.y - 32, hub.x - 40, hub.y + 30),
      stroke: '#546e7a', width: 1.5, dash: '6 4', marker: 'url(#arrow-host)',
    })
  }
  ;(props.mockup?.spec?.clusters || []).forEach((c) => {
    const n = byId[c.id]
    if (infra && n) {
      out.push({
        id: `infra-${c.id}`,
        d: curve(infra.x + 90, infra.y - 20, n.x - 40, n.y + 30),
        stroke: '#546e7a', width: 1.25, dash: '6 4', marker: 'url(#arrow-host)',
      })
    }
  })

  // VyOS LAN attaches lab guests
  if (gw && hub) {
    out.push({
      id: 'gw-hub',
      d: curve(gw.x + 70, gw.y, hub.x - 70, hub.y),
      stroke: '#ef6c00', width: 2.5, dash: null, marker: 'url(#arrow)',
    })
  }
  ;(props.mockup?.spec?.clusters || []).forEach((c) => {
    const n = byId[c.id]
    if (gw && n) {
      out.push({
        id: `gw-${c.id}`,
        d: curve(gw.x + 60, gw.y + 10, n.x - 90, n.y + 10),
        stroke: '#ef6c00', width: 1.75, dash: '4 3', marker: 'url(#arrow)',
      })
    }
  })

  if (hub && acm) {
    out.push({
      id: 'hub-acm',
      d: curve(hub.x, hub.y - 30, acm.x, acm.y + 30),
      stroke: '#78909c', width: 2.5, dash: null, marker: 'url(#arrow)',
    })
  }
  ;(props.mockup?.spec?.clusters || []).forEach((c) => {
    const n = byId[c.id]
    if (acm && n) {
      out.push({
        id: `acm-${c.id}`,
        d: curve(acm.x + 60, acm.y, n.x - 100, n.y),
        stroke: '#78909c', width: 2.5, dash: null, marker: 'url(#arrow)',
      })
    }
  })
  return out
})

function curve(x1, y1, x2, y2) {
  const mx = (x1 + x2) / 2
  const my = (y1 + y2) / 2
  return `M ${x1} ${y1} Q ${mx} ${my} ${x2} ${y2}`
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
    x: Math.max(70, Math.min(vbW - 70, dragging.value.sx + dx)),
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
  min-height: 540px;
}
.topo-svg {
  width: 100%;
  height: 540px;
  display: block;
  cursor: grab;
}
.topo-node { cursor: pointer; }
.topo-node.active rect { filter: brightness(1.05); }
.fill-infra { fill: #37474f; }
.fill-gateway { fill: #e65100; }
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
  fill: rgba(255, 255, 255, 0.88);
  font-size: 10px;
  pointer-events: none;
}
.link { pointer-events: none; }
</style>
