/** Guest VMs the adapter provisions for a MockUp (synthetic infra nodes). */
export function enumerateVHosts(m) {
  if (!m?.spec) return []
  const out = []
  if (m.spec.gateway?.id) {
    out.push({
      id: 'vhost-gw',
      role: 'gw',
      parentKind: 'gateway',
      parentId: m.spec.gateway.id,
      label: 'vHost-GW',
      sub: 'RTR guest',
    })
  }
  if (m.spec.hub?.id) {
    out.push({
      id: 'vhost-hub-0',
      role: 'hub',
      parentKind: 'hub',
      parentId: m.spec.hub.id,
      label: 'vHost-MGMT',
      sub: 'OCP SNO guest',
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
        sub: `${c.label || c.name} guest`,
        clusterIndex: ci,
        nodeIndex: i,
      })
    }
  })
  return out
}
