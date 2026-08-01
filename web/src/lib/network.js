/** Lab L2/L3 picture: vSwitch, guest vNICs, VyOS WAN/LAN, PRI-PHY-NIC on MACHINE-HOST. */

import { enumerateVHosts } from './vhosts'

function guestIP(m, vh) {
  if (vh.role === 'gw') return m.spec.gateway?.lanIP || m.spec.network?.gateway || '10.77.30.1'
  if (vh.role === 'hub') return m.spec.hub?.ip || '—'
  if (vh.role === 'deployment') {
    const c = (m.spec.clusters || []).find((x) => x.id === vh.parentId)
    if (!c?.ipBase) return '—'
    // ipBase is master-0; + nodeIndex
    const parts = String(c.ipBase).split('.')
    if (parts.length !== 4) return c.ipBase
    const last = Number(parts[3]) + (vh.nodeIndex || 0)
    return `${parts[0]}.${parts[1]}.${parts[2]}.${last}`
  }
  return '—'
}

/**
 * Synthetic network objects for the Network layer view.
 * Edges: PRI-PHY-NIC ↔ VyOS eth0 WAN; eth1 LAN + all guest vNICs ↔ vSwitch.
 */
export function enumerateNetwork(m) {
  if (!m?.spec) return { nodes: [], meta: {} }
  const canvas = m.spec.canvas || {}
  const cidr = m.spec.gateway?.lanCIDR || m.spec.network?.machineCIDR || '10.77.30.0/24'
  const gwIP = m.spec.gateway?.lanIP || m.spec.network?.gateway || '10.77.30.1'
  const lanNet = m.spec.gateway?.lanNetwork || m.spec.infraHost?.networkName || 'ocp-lab'
  const provider = m.spec.provider || 'libvirt'

  const nodes = []

  if (m.spec.infraHost?.id && !canvas.omitHost) {
    nodes.push({
      id: m.spec.infraHost.id,
      kind: 'infraHost',
      label: m.spec.infraHost.label || 'MACHINE-HOST',
      sub: `HW · ${m.spec.infraHost.os || 'rhel'}`,
      cls: 'fill-infra',
      w: 180,
      h: 48,
    })
    nodes.push({
      id: 'phy-pri',
      kind: 'phyNic',
      label: 'PRI-PHY-NIC',
      sub: 'bridged uplink',
      cls: 'fill-phy',
      w: 130,
      h: 44,
      facts: {
        role: 'uplink',
        mode: 'bridged',
        attaches: 'VyOS eth0 WAN',
      },
    })
  }

  nodes.push({
    id: 'vswitch',
    kind: 'vswitch',
    label: `vSwitch · ${lanNet}`,
    sub: `${provider} · ${cidr}`,
    cls: 'fill-vswitch',
    w: 220,
    h: 52,
    facts: {
      network: lanNet,
      cidr,
      gateway: gwIP,
    },
  })

  if (m.spec.gateway?.id && !canvas.omitGateway) {
    nodes.push({
      id: 'vnic-gw-wan',
      kind: 'vnic',
      label: 'eth0 WAN',
      sub: 'VyOS → PRI-PHY',
      role: 'wan',
      parentVHost: 'vhost-gw',
      cls: 'fill-vnic-wan',
      w: 110,
      h: 42,
      facts: { iface: 'eth0', role: 'wan', peer: 'PRI-PHY-NIC' },
    })
    nodes.push({
      id: 'vnic-gw-lan',
      kind: 'vnic',
      label: 'eth1 LAN',
      sub: `GW ${gwIP}`,
      role: 'lan',
      parentVHost: 'vhost-gw',
      ip: gwIP,
      cls: 'fill-vnic-lan',
      w: 110,
      h: 42,
      facts: { iface: 'eth1', role: 'lan', ip: gwIP, cidr },
    })
  }

  for (const vh of enumerateVHosts(m)) {
    if (vh.role === 'gw') continue // covered by eth0/eth1
    const ip = guestIP(m, vh)
    nodes.push({
      id: `vnic-${vh.id}`,
      kind: 'vnic',
      label: `${vh.label} · eth0`,
      sub: ip !== '—' ? ip : 'guest NIC',
      role: 'guest',
      parentVHost: vh.id,
      ip,
      cls: 'fill-vnic',
      w: 118,
      h: 42,
      facts: { iface: 'eth0', role: 'guest', ip, vHost: vh.label },
    })
  }

  return {
    nodes,
    meta: { cidr, gwIP, lanNet, provider },
  }
}
