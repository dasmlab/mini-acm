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
        <marker id="arrow-cluster" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
          <path d="M0,0 L6,3 L0,6 Z" fill="#0277bd" />
        </marker>
        <marker id="arrow-hub" markerWidth="8" markerHeight="8" refX="6" refY="3" orient="auto">
          <path d="M0,0 L6,3 L0,6 Z" fill="#1565c0" />
        </marker>
        <filter id="softShadow" x="-40%" y="-40%" width="180%" height="180%">
          <feDropShadow dx="0" dy="3" stdDeviation="4" flood-color="#0b1f3a" flood-opacity="0.18" />
        </filter>
        <clipPath
          v-for="n in nodes"
          :id="`node-clip-${n.id}`"
          :key="`clip-${n.id}`"
        >
          <rect
            :x="n.x - n.w / 2 + 5"
            :y="n.y - n.h / 2 + 3"
            :width="n.w - 10"
            :height="n.h - 6"
            :rx="Math.max(2, (n.rx || 8) - 2)"
          />
        </clipPath>
      </defs>

      <rect width="100%" height="100%" fill="url(#grid)" />

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
        <g :clip-path="`url(#node-clip-${n.id})`">
          <text :x="n.x" :y="n.y - 7" text-anchor="middle" class="node-title">{{ n.label }}</text>
          <text :x="n.x" :y="n.y + 11" text-anchor="middle" class="node-sub">{{ n.sub }}</text>
        </g>
      </g>
    </svg>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { enumerateVHosts, enumerateAppliances } from 'src/lib/vhosts'
import { enumerateNetwork } from 'src/lib/network'

const props = defineProps({
  mockup: { type: Object, required: true },
  selectedId: { type: String, default: null },
  /** all | infra | network | cluster | app */
  layer: { type: String, default: 'all' },
  /** free-form: no constrained edges unless showRelations */
  freeForm: { type: Boolean, default: false },
})
const emit = defineEmits(['select', 'move'])

const vbW = 960
const vbH = 680
const dragging = ref(null)
const moved = ref(false)

/**
 * Infra = host + adapter + guest vHosts (+ VyOS NF on the GW vHost).
 * Network = vSwitch + vNICs + PRI-PHY-NIC (WAN/LAN).
 * Cluster = OCP objects + ACM governance.
 * App = ACM payload only.
 */
const VIEW_KINDS = {
  infra: new Set(['infraHost', 'adapter', 'vhost', 'gateway', 'appliance']),
  network: new Set(['infraHost', 'phyNic', 'vswitch', 'vnic']),
  cluster: new Set(['hub', 'cluster', 'acm']),
  app: new Set(['acm']),
}

const showRelations = computed(() => {
  if (!props.freeForm) return true
  return !!props.mockup?.spec?.canvas?.showRelations
})

const bands = computed(() => {
  if (props.layer === 'infra') {
    return [{ id: 'infra', y: 0, h: vbH, fill: '#eceff1', label: props.freeForm ? 'FREE-FORM · drop vHosts / appliances' : 'INFRASTRUCTURE · host · adapter · vHosts (incl. GW) · VyOS on vHost-GW' }]
  }
  if (props.layer === 'network') {
    return [{ id: 'network', y: 0, h: vbH, fill: '#fff3e0', label: 'NETWORK · PRI-PHY-NIC · VyOS WAN/LAN · vSwitch · guest vNICs' }]
  }
  if (props.layer === 'cluster') {
    return [{ id: 'cluster', y: 0, h: vbH, fill: '#e3f2fd', label: 'CLUSTER MGMT · ACM on home · governs deployments' }]
  }
  if (props.layer === 'app') {
    return [{ id: 'app', y: 0, h: vbH, fill: '#e0f2f1', label: 'APPLICATION · ACM payload (others later)' }]
  }
  return [
    { id: 'app', y: 0, h: 100, fill: '#e0f2f1', label: 'APP · ACM payload' },
    { id: 'cluster', y: 100, h: 140, fill: '#e3f2fd', label: 'CLUSTER · OCP (mgmt + deployments)' },
    { id: 'infra', y: 240, h: 440, fill: '#eceff1', label: props.freeForm ? 'INFRA · free-form' : 'INFRA · host · adapter · vHosts · VyOS on GW' },
  ]
})

const pos = (id, fallback) => {
  const p = props.mockup?.layout?.nodes?.[id]
  return p ? { x: p.x, y: p.y } : fallback
}

function inView(kind) {
  if (props.layer === 'all') {
    // Full rack stays high-level — detailed NICs live in Network tab
    return !['phyNic', 'vswitch', 'vnic'].includes(kind)
  }
  return VIEW_KINDS[props.layer]?.has(kind) === true
}

const allNodes = computed(() => {
  const m = props.mockup
  if (!m?.spec) return []
  const L = props.layer

  // Dedicated Network layer layout
  if (L === 'network') {
    return layoutNetworkNodes(m)
  }

  const out = []
  const provider = m.spec.provider || 'libvirt'
  const showInfraStuff = L === 'all' || L === 'infra'
  const showClusterStuff = L === 'all' || L === 'cluster'
  const showAppStuff = L === 'all' || L === 'app' || L === 'cluster'

  const yHost = L === 'infra' ? 580 : 600
  const yAdapter = L === 'infra' ? 480 : 520
  const yVHost = L === 'infra' ? 320 : 400
  const yVyOS = L === 'infra' ? 220 : 310
  const yOCP = L === 'cluster' ? 280 : 180
  const yACM = L === 'app' || L === 'cluster' ? 120 : 55

  if (showInfraStuff) {
    const canvas = m.spec.canvas || {}
    const infra = m.spec.infraHost
    if (infra?.id && !canvas.omitHost) {
      const ip = pos(infra.id, { x: 120, y: yHost })
      out.push({
        id: infra.id, kind: 'infraHost',
        label: infra.label || 'MACHINE-HOST',
        sub: `HW · ${infra.os || 'rhel'}`,
        x: ip.x, y: ip.y, w: 200, h: 56, rx: 8, cls: 'fill-infra',
      })
    }

    if (!canvas.omitHost) {
      const ap = pos('adapter', { x: 360, y: yAdapter })
      out.push({
        id: 'adapter', kind: 'adapter',
        label: 'ADAPTER',
        sub: `${provider} · IaaS`,
        x: ap.x, y: ap.y, w: 150, h: 50, rx: 8, cls: 'fill-adapter',
      })
    }

    const vhosts = enumerateVHosts(m)
    const spacing = Math.min(118, Math.floor((vbW - 40) / Math.max(vhosts.length, 1)))
    const startX = Math.max(70, (vbW - (vhosts.length - 1) * spacing) / 2)
    vhosts.forEach((vh, i) => {
      const orphan = (m.spec.canvas?.orphans || []).find((o) => o.id === vh.id)
      const fallback = orphan?.x
        ? { x: orphan.x, y: orphan.y }
        : { x: startX + i * spacing, y: yVHost }
      const p = pos(vh.id, fallback)
      const labelLen = Math.max((vh.label || '').length, (vh.sub || '').length)
      const w = Math.min(132, Math.max(108, 7.2 * labelLen + 20))
      out.push({
        id: vh.id,
        kind: 'vhost',
        role: vh.role,
        parentKind: vh.parentKind,
        parentId: vh.parentId,
        label: vh.label,
        sub: vh.sub,
        x: p.x, y: p.y, w, h: 52, rx: 8, cls: 'fill-vhost',
      })
    })

    enumerateAppliances(m).forEach((apn, i) => {
      const under = out.find((n) => n.id === apn.runsOn)
      const fallback = under
        ? { x: under.x, y: under.y - 68 }
        : { x: 200 + i * 120, y: yVyOS }
      const orphan = (m.spec.canvas?.orphans || []).find((o) => o.id === apn.id)
      const p = orphan?.x ? { x: orphan.x, y: orphan.y } : pos(apn.id, fallback)
      const x = under && Math.abs(p.x - under.x) < 8 ? under.x : p.x
      out.push({
        id: apn.id,
        kind: apn.kind === 'gateway' ? 'gateway' : 'appliance',
        label: apn.label,
        sub: apn.applianceType === 'vyos' ? 'RTR on vHost-GW' : (apn.applianceType || 'appliance'),
        runsOn: apn.runsOn,
        x, y: p.y, w: 120, h: 46, rx: 10,
        cls: apn.applianceType === 'haproxy' ? 'fill-hap' : (apn.kind === 'gateway' ? 'fill-gateway' : 'fill-appliance'),
      })
    })
  }

  if (showClusterStuff) {
    const canvas = m.spec.canvas || {}
    const hub = m.spec.hub
    if (hub?.id && !canvas.omitHub) {
      const mgmtVh = out.find((n) => n.kind === 'vhost' && n.parentKind === 'hub')
        || out.find((n) => n.id === 'vhost-hub-0')
      const hp = pos(hub.id, { x: mgmtVh?.x || 260, y: yOCP })
      out.push({
        id: hub.id, kind: 'hub',
        label: hub.label || (L === 'all' && m.spec.style === 'single-sno-ocp' ? 'OCP-MGMT' : 'MGMT-CLUSTER'),
        sub: m.spec.style === 'single-sno-ocp'
          ? (L === 'all' ? 'SNO · OCP' : 'SNO · single node OCP')
          : (L === 'all' ? 'home · hosts ACM' : 'home · 1× SNO · hosts ACM'),
        x: hp.x, y: hp.y, w: 200, h: 52, rx: 12, cls: 'fill-hub',
      })
    }
    ;(m.spec.clusters || []).forEach((c, i) => {
      const n = Math.max(1, Number(c.count) || 3)
      const kids = out.filter((vh) => vh.kind === 'vhost' && vh.parentId === c.id)
      const avgX = kids.length
        ? kids.reduce((s, vh) => s + vh.x, 0) / kids.length
        : 560 + (i % 2) * 30
      const cp = pos(c.id, { x: avgX, y: yOCP - 20 + i * 70 })
      out.push({
        id: c.id, kind: 'cluster',
        label: c.label || c.name,
        sub: L === 'all' ? 'managed OCP' : `managed · ${n}× cp/worker VMs`,
        x: cp.x, y: cp.y, w: 210, h: 52, rx: 12, cls: 'fill-cluster',
        clusterIndex: i,
      })
    })
  }

  if (showAppStuff) {
    const canvas = m.spec.canvas || {}
    const acm = m.spec.acm
    if (acm?.id && !canvas.omitACM) {
      const acp = pos(acm.id, { x: 260, y: yACM })
      out.push({
        id: acm.id, kind: 'acm',
        label: acm.label || 'ACM',
        sub: acm.enabled ? 'payload · governs' : 'off',
        x: acp.x, y: acp.y, w: 140, h: 52, rx: 12, cls: 'fill-acm',
      })
    }
  }

  return out
})

function layoutNetworkNodes(m) {
  const { nodes: net } = enumerateNetwork(m)
  const out = []
  const guestVnics = net.filter((n) => n.kind === 'vnic' && n.role === 'guest')
  const spacing = Math.min(115, Math.floor((vbW - 80) / Math.max(guestVnics.length + 2, 1)))

  // Bottom: host + PRI-PHY-NIC
  const host = net.find((n) => n.kind === 'infraHost')
  if (host) {
    const p = pos(host.id, { x: 140, y: 600 })
    out.push({ ...host, x: p.x, y: p.y, rx: 8, w: host.w || 180, h: host.h || 48 })
  }
  const phy = net.find((n) => n.kind === 'phyNic')
  if (phy) {
    const p = pos(phy.id, { x: 340, y: 600 })
    out.push({ ...phy, x: p.x, y: p.y, rx: 6, w: phy.w || 130, h: phy.h || 44 })
  }

  // Middle-low: VyOS WAN
  const wan = net.find((n) => n.id === 'vnic-gw-wan')
  if (wan) {
    const p = pos(wan.id, { x: 340, y: 480 })
    out.push({ ...wan, x: p.x, y: p.y, rx: 6, w: wan.w || 110, h: wan.h || 42 })
  }

  // Middle: vSwitch bar
  const vs = net.find((n) => n.kind === 'vswitch')
  if (vs) {
    const p = pos(vs.id, { x: 480, y: 340 })
    out.push({ ...vs, x: p.x, y: p.y, rx: 8, w: Math.min(280, 160 + guestVnics.length * 20), h: vs.h || 52 })
  }

  // Middle-high: VyOS LAN (.1 GW)
  const lan = net.find((n) => n.id === 'vnic-gw-lan')
  if (lan) {
    const p = pos(lan.id, { x: 160, y: 220 })
    out.push({ ...lan, x: p.x, y: p.y, rx: 6, w: lan.w || 110, h: lan.h || 42 })
  }

  // Top: guest vNICs
  const startX = 280
  guestVnics.forEach((vn, i) => {
    const p = pos(vn.id, { x: startX + i * spacing, y: 140 })
    out.push({ ...vn, x: p.x, y: p.y, rx: 6, w: vn.w || 118, h: vn.h || 42 })
  })

  return out
}

const nodes = computed(() => allNodes.value.filter((n) => inView(n.kind)))

const edges = computed(() => {
  if (!showRelations.value && props.layer !== 'network') return []

  const byId = Object.fromEntries(nodes.value.map((n) => [n.id, n]))
  const out = []
  const L = props.layer

  if (L === 'network') {
    const phy = byId['phy-pri']
    const host = byId[props.mockup?.spec?.infraHost?.id]
    const wan = byId['vnic-gw-wan']
    const lan = byId['vnic-gw-lan']
    const vs = byId.vswitch
    if (host && phy) {
      out.push({
        id: 'host-phy',
        d: curve(host.x + 80, host.y, phy.x - 60, phy.y),
        stroke: '#37474f', width: 2, dash: null, marker: 'url(#arrow-host)',
      })
    }
    if (phy && wan) {
      out.push({
        id: 'phy-wan',
        d: curve(phy.x, phy.y - 22, wan.x, wan.y + 22),
        stroke: '#e65100', width: 2.25, dash: null, marker: 'url(#arrow)',
      })
    }
    if (wan && lan) {
      // both on same VyOS box conceptually
      out.push({
        id: 'wan-lan-rtr',
        d: curve(wan.x - 40, wan.y - 10, lan.x + 20, lan.y + 20),
        stroke: '#c62828', width: 1.5, dash: '4 3', marker: null,
      })
    }
    if (lan && vs) {
      out.push({
        id: 'lan-vswitch',
        d: curve(lan.x + 40, lan.y + 20, vs.x - 80, vs.y - 10),
        stroke: '#ef6c00', width: 2, dash: null, marker: 'url(#arrow)',
      })
    }
    nodes.value.filter((n) => n.kind === 'vnic' && n.role === 'guest').forEach((vn) => {
      if (!vs) return
      out.push({
        id: `vs-${vn.id}`,
        d: curve(vs.x, vs.y - 26, vn.x, vn.y + 22),
        stroke: '#fb8c00', width: 1.5, dash: '5 3', marker: 'url(#arrow)',
      })
    })
    return out
  }

  const infra = byId[props.mockup?.spec?.infraHost?.id]
  const adapter = byId.adapter
  const gwVHost = byId['vhost-gw']
  const hub = byId[props.mockup?.spec?.hub?.id]
  const acm = byId[props.mockup?.spec?.acm?.id]

  if (L === 'all' || L === 'infra') {
    if (infra && adapter) {
      out.push({
        id: 'infra-adapter',
        d: curve(infra.x + 90, infra.y - 20, adapter.x - 60, adapter.y + 10),
        stroke: '#546e7a', width: 2, dash: '5 3', marker: 'url(#arrow-host)',
      })
    }
    nodes.value.filter((n) => n.kind === 'vhost').forEach((vh) => {
      if (!adapter) return
      out.push({
        id: `adapter-${vh.id}`,
        d: curve(adapter.x, adapter.y - 24, vh.x, vh.y + 26),
        stroke: '#546e7a', width: 1.35, dash: '5 3', marker: 'url(#arrow-host)',
      })
    })
    nodes.value.filter((n) => n.kind === 'gateway' || n.kind === 'appliance').forEach((apn) => {
      const under = byId[apn.runsOn] || (apn.kind === 'gateway' ? gwVHost : null)
      if (apn && under) {
        out.push({
          id: `payload-${apn.id}`,
          d: `M ${apn.x} ${apn.y + 24} L ${under.x} ${under.y - 26}`,
          stroke: '#c62828', width: 2.25, dash: null, marker: 'url(#arrow)',
        })
      }
    })
  }

  // Full rack: OCP / cluster payload sits on its vHost(s) — same “down” idea as VyOS → vHost-GW.
  if (L === 'all') {
    if (hub) {
      const mgmt = nodes.value.find((n) => n.kind === 'vhost' && n.parentKind === 'hub')
        || byId['vhost-hub-0']
      if (mgmt) {
        out.push({
          id: 'hub-vhost-mgmt',
          d: `M ${hub.x} ${hub.y + 26} L ${mgmt.x} ${mgmt.y - 26}`,
          stroke: '#1565c0', width: 2.25, dash: null, marker: 'url(#arrow-hub)',
        })
      }
    }
    nodes.value.filter((n) => n.kind === 'cluster').forEach((c) => {
      nodes.value
        .filter((vh) => vh.kind === 'vhost' && vh.parentId === c.id)
        .forEach((vh) => {
          out.push({
            id: `cluster-vhost-${c.id}-${vh.id}`,
            d: `M ${c.x} ${c.y + 26} L ${vh.x} ${vh.y - 26}`,
            stroke: '#0277bd', width: 1.85, dash: null, marker: 'url(#arrow-cluster)',
          })
        })
    })
  }

  if (L === 'all' || L === 'cluster' || L === 'app') {
    if (hub && acm) {
      out.push({
        id: 'hub-acm',
        d: curve(hub.x, hub.y - 26, acm.x, acm.y + 26),
        stroke: '#00838f', width: 2.5, dash: null, marker: 'url(#arrow)',
      })
    }
  }

  if (L === 'all' || L === 'cluster') {
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
    x: Math.max(50, Math.min(vbW - 50, dragging.value.sx + dx)),
    y: Math.max(36, Math.min(vbH - 36, dragging.value.sy + dy)),
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
  min-height: 640px;
}
.topo-svg {
  width: 100%;
  height: 640px;
  display: block;
  cursor: grab;
}
.topo-node { cursor: pointer; }
.topo-node.active rect { filter: brightness(1.05); }
.fill-infra { fill: #37474f; }
.fill-adapter { fill: #546e7a; }
.fill-vhost { fill: #607d8b; }
.fill-gateway { fill: #c62828; }
.fill-appliance { fill: #6d4c41; }
.fill-hap { fill: #5d4037; }
.fill-hub { fill: #1a237e; }
.fill-acm { fill: #00838f; }
.fill-cluster { fill: #1565c0; }
.fill-vswitch { fill: #ef6c00; }
.fill-phy { fill: #455a64; }
.fill-vnic { fill: #ffa726; }
.fill-vnic-wan { fill: #e65100; }
.fill-vnic-lan { fill: #fb8c00; }
.band-label {
  fill: #607d8b;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.05em;
  pointer-events: none;
}
.node-title {
  fill: #fff;
  font-size: 11px;
  font-weight: 700;
  pointer-events: none;
}
.node-sub {
  fill: rgba(255, 255, 255, 0.92);
  font-size: 9px;
  font-weight: 500;
  pointer-events: none;
}
.link { pointer-events: none; }
</style>
