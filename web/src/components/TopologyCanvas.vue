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

      <!-- Layer bands (all / or single-layer wash) -->
      <g v-for="b in bands" :key="b.id">
        <rect :x="0" :y="b.y" :width="vbW" :height="b.h" :fill="b.fill" opacity="0.55" />
        <text :x="14" :y="b.y + 18" class="band-label">{{ b.label }}</text>
      </g>

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
        :class="{ active: selectedId === n.id, dim: n.dim }"
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
  /** all | infra | cluster */
  layer: { type: String, default: 'all' },
})
const emit = defineEmits(['select', 'move'])

const vbW = 780
const vbH = 600
const dragging = ref(null)
const moved = ref(false)

/** Views are not a strict stack: hub + deployment guests appear in both infra (VMs) and cluster (OCP). Gateway is Full-rack only (egress). */
const VIEW_KINDS = {
  infra: new Set(['infraHost', 'adapter', 'hub', 'cluster']),
  cluster: new Set(['hub', 'cluster', 'acm']),
}

const bands = computed(() => {
  if (props.layer === 'infra') {
    return [{ id: 'infra', y: 0, h: vbH, fill: '#eceff1', label: 'INFRASTRUCTURE · host · adapter · guest VMs' }]
  }
  if (props.layer === 'cluster') {
    return [{ id: 'cluster', y: 0, h: vbH, fill: '#e3f2fd', label: 'CLUSTER MGMT · ACM on home · governs deployments' }]
  }
  return [
    { id: 'cluster', y: 0, h: 280, fill: '#e3f2fd', label: 'CLUSTER MGMT · ACM · OCP' },
    { id: 'infra', y: 280, h: 320, fill: '#eceff1', label: 'INFRASTRUCTURE · host · adapter · VMs · GW (egress)' },
  ]
})

const pos = (id, fallback) => {
  const p = props.mockup?.layout?.nodes?.[id]
  return p ? { x: p.x, y: p.y } : fallback
}

const layerDefaultY = {
  acm: 70,
  cluster: 200,
  infra: 470,
}

function inView(kind) {
  if (props.layer === 'all') return true
  return VIEW_KINDS[props.layer]?.has(kind) === true
}

const allNodes = computed(() => {
  const m = props.mockup
  if (!m?.spec) return []
  const out = []
  const provider = m.spec.provider || 'libvirt'
  const L = props.layer
  const infraMode = L === 'infra'

  const infra = m.spec.infraHost
  if (infra?.id) {
    const ip = pos(infra.id, { x: 140, y: layerDefaultY.infra })
    out.push({
      id: infra.id, kind: 'infraHost',
      label: infra.label || 'MACHINE-HOST',
      sub: `HW · ${infra.os || 'rhel'}`,
      x: ip.x, y: ip.y, w: 200, h: 60, rx: 8, cls: 'fill-infra',
    })
  }

  const ap = pos('adapter', { x: 360, y: layerDefaultY.infra + 10 })
  out.push({
    id: 'adapter', kind: 'adapter',
    label: 'ADAPTER',
    sub: `${provider} · IaaS`,
    x: ap.x, y: ap.y, w: 150, h: 54, rx: 8, cls: 'fill-adapter',
  })

  const gw = m.spec.gateway
  if (gw?.id) {
    const gp = pos(gw.id, { x: 560, y: layerDefaultY.infra })
    out.push({
      id: gw.id, kind: 'gateway',
      label: gw.label || 'VYOS-GW',
      sub: `egress · ${gw.lanCIDR || 'LAN'}`,
      x: gp.x, y: gp.y, w: 160, h: 56, rx: 10, cls: 'fill-gateway',
    })
  }

  const hub = m.spec.hub
  if (hub?.id) {
    const hubY = infraMode ? 220 : (L === 'cluster' ? 260 : layerDefaultY.cluster)
    const hp = pos(hub.id, { x: 220, y: hubY })
    out.push({
      id: hub.id, kind: 'hub',
      label: hub.label || 'MGMT-CLUSTER',
      sub: infraMode ? 'guest VM · OCP OS' : 'home · hosts ACM',
      x: hp.x, y: hp.y, w: 188, h: 56, rx: 12, cls: 'fill-hub',
    })
  }

  ;(m.spec.clusters || []).forEach((c, i) => {
    const baseY = infraMode ? 220 : (L === 'cluster' ? 260 : layerDefaultY.cluster)
    const cp = pos(c.id, { x: 480 + (i % 2) * 40, y: baseY + (i * 70) })
    out.push({
      id: c.id, kind: 'cluster',
      label: c.label || c.name,
      sub: infraMode ? 'guest VMs · OCP OS' : 'managed OCP',
      x: cp.x, y: cp.y, w: 200, h: 56, rx: 12, cls: 'fill-cluster',
      clusterIndex: i,
    })
  })

  const acm = m.spec.acm
  if (acm?.id) {
    const acmY = L === 'cluster' ? 110 : layerDefaultY.acm
    const acp = pos(acm.id, { x: 220, y: acmY })
    out.push({
      id: acm.id, kind: 'acm',
      label: acm.label || 'ACM',
      sub: acm.enabled ? 'governs spokes' : 'off',
      x: acp.x, y: acp.y, w: 140, h: 56, rx: 12, cls: 'fill-acm',
    })
  }

  return out
})

const nodes = computed(() => allNodes.value.filter((n) => inView(n.kind)))

const edges = computed(() => {
  const byId = Object.fromEntries(nodes.value.map((n) => [n.id, n]))
  const infra = byId[props.mockup?.spec?.infraHost?.id]
  const adapter = byId.adapter
  const gw = byId[props.mockup?.spec?.gateway?.id]
  const hub = byId[props.mockup?.spec?.hub?.id]
  const acm = byId[props.mockup?.spec?.acm?.id]
  const out = []
  const L = props.layer

  // Infra: host ↔ adapter ↔ guest VMs (provisioning). No ACM edges.
  if (L === 'all' || L === 'infra') {
    if (infra && adapter) {
      out.push({
        id: 'infra-adapter',
        d: curve(infra.x + 90, infra.y, adapter.x - 70, adapter.y),
        stroke: '#546e7a', width: 2, dash: '5 3', marker: 'url(#arrow-host)',
      })
    }
    if (adapter && hub) {
      out.push({
        id: 'adapter-hub',
        d: curve(adapter.x, adapter.y - 28, hub.x, hub.y + 28),
        stroke: '#546e7a', width: 1.5, dash: '6 4', marker: 'url(#arrow-host)',
      })
    }
    ;(props.mockup?.spec?.clusters || []).forEach((c) => {
      const n = byId[c.id]
      if (adapter && n) {
        out.push({
          id: `adapter-${c.id}`,
          d: curve(adapter.x + 40, adapter.y - 28, n.x - 40, n.y + 28),
          stroke: '#546e7a', width: 1.25, dash: '6 4', marker: 'url(#arrow-host)',
        })
      }
    })
  }

  // Full rack only: GW / egress / vSwitch path
  if (L === 'all') {
    if (adapter && gw) {
      out.push({
        id: 'adapter-gw',
        d: curve(adapter.x + 70, adapter.y, gw.x - 70, gw.y),
        stroke: '#546e7a', width: 1.5, dash: '5 3', marker: 'url(#arrow-host)',
      })
    }
    if (infra && gw) {
      out.push({
        id: 'infra-gw',
        d: curve(infra.x, infra.y - 30, gw.x - 40, gw.y + 20),
        stroke: '#78909c', width: 1.25, dash: '4 3', marker: 'url(#arrow-host)',
      })
    }
    if (gw && hub) {
      out.push({
        id: 'gw-hub',
        d: curve(gw.x - 40, gw.y - 28, hub.x + 40, hub.y + 28),
        stroke: '#ef6c00', width: 2, dash: null, marker: 'url(#arrow)',
      })
    }
    ;(props.mockup?.spec?.clusters || []).forEach((c) => {
      const n = byId[c.id]
      if (gw && n) {
        out.push({
          id: `gw-${c.id}`,
          d: curve(gw.x, gw.y - 28, n.x, n.y + 28),
          stroke: '#ef6c00', width: 1.5, dash: '4 3', marker: 'url(#arrow)',
        })
      }
    })
  }

  // Cluster mgmt: ACM on home → governs deployments (not self-mgmt yet)
  if (L === 'all' || L === 'cluster') {
    if (hub && acm) {
      out.push({
        id: 'hub-acm',
        d: curve(hub.x, hub.y - 28, acm.x, acm.y + 28),
        stroke: '#00838f', width: 2.5, dash: null, marker: 'url(#arrow)',
      })
    }
    ;(props.mockup?.spec?.clusters || []).forEach((c) => {
      const n = byId[c.id]
      if (acm && n) {
        out.push({
          id: `acm-${c.id}`,
          d: curve(acm.x + 60, acm.y + 10, n.x - 80, n.y - 10),
          stroke: '#00838f', width: 2, dash: null, marker: 'url(#arrow)',
        })
      }
    })
  }

  return out
})

function curve(x1, y1, x2, y2) {
  const mx = (x1 + x2) / 2
  const my = (y1 + y2) / 2
  return `M ${x1} ${y1} Q ${mx} ${my} ${x2} ${y2}`
}

function onDown(ev, n) {
  if (n.kind === 'adapter') {
    // still allow select; drag position stored under layout.nodes.adapter
  }
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
  min-height: 560px;
}
.topo-svg {
  width: 100%;
  height: 560px;
  display: block;
  cursor: grab;
}
.topo-node { cursor: pointer; }
.topo-node.active rect { filter: brightness(1.05); }
.topo-node.dim { opacity: 0.35; }
.fill-infra { fill: #37474f; }
.fill-adapter { fill: #546e7a; }
.fill-gateway { fill: #e65100; }
.fill-hub { fill: #1a237e; }
.fill-acm { fill: #00838f; }
.fill-cluster { fill: #1565c0; }
.band-label {
  fill: #90a4ae;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.06em;
  pointer-events: none;
}
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
