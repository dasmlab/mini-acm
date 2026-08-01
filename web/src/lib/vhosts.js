/** Guest VMs the adapter provisions for a MockUp (synthetic infra nodes). */
export function enumerateVHosts(m) {
  if (!m?.spec) return []
  const canvas = m.spec.canvas || {}
  const out = []
  if (m.spec.gateway?.id && !canvas.omitGateway) {
    out.push({
      id: 'vhost-gw',
      role: 'gw',
      parentKind: 'gateway',
      parentId: m.spec.gateway.id,
      label: 'vHost-GW',
      sub: '1× RTR guest',
    })
  }
  if (m.spec.hub?.id && !canvas.omitHub) {
    out.push({
      id: 'vhost-hub-0',
      role: 'hub',
      parentKind: 'hub',
      parentId: m.spec.hub.id,
      label: 'vHost-MGMT',
      sub: '1× SNO · OCP',
    })
  }
  ;(m.spec.clusters || []).forEach((c, ci) => {
    const n = Math.max(1, Number(c.count) || 3)
    const short = `D${ci + 1}`
    for (let i = 0; i < n; i++) {
      out.push({
        id: `vhost-${c.id}-${i}`,
        role: 'deployment',
        parentKind: 'cluster',
        parentId: c.id,
        label: `vHost-${short}-${i}`,
        sub: `cp/worker · ${i + 1}/${n}`,
        clusterIndex: ci,
        nodeIndex: i,
      })
    }
  })
  for (const o of canvas.orphans || []) {
    if (o.kind !== 'vhost') continue
    out.push({
      id: o.id,
      role: 'orphan',
      parentKind: 'orphan',
      parentId: null,
      label: o.label || o.id,
      sub: 'free-form vHost',
      orphan: true,
    })
  }
  return out
}

export function enumerateAppliances(m) {
  const out = []
  const canvas = m?.spec?.canvas || {}
  if (m?.spec?.gateway?.id && !canvas.omitGateway) {
    out.push({
      id: m.spec.gateway.id,
      kind: 'gateway',
      label: m.spec.gateway.label || 'VYOS-GW',
      applianceType: 'vyos',
      runsOn: 'vhost-gw',
    })
  }
  for (const o of canvas.orphans || []) {
    if (o.kind !== 'appliance') continue
    out.push({
      id: o.id,
      kind: 'appliance',
      label: o.label || o.id,
      applianceType: o.applianceType || 'other',
      runsOn: o.runsOn || '',
      orphan: true,
    })
  }
  return out
}

export function ensureCanvas(m) {
  if (!m.spec.canvas) {
    m.spec.canvas = { orphans: [], showRelations: false }
  }
  if (!m.spec.canvas.orphans) m.spec.canvas.orphans = []
  return m.spec.canvas
}

export function newOrphanId(prefix) {
  return `${prefix}-${Math.random().toString(36).slice(2, 8)}`
}
