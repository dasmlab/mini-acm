import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: 30_000,
  withCredentials: true,
})

export default api

export async function getHealth() {
  const { data } = await api.get('/health')
  return data
}

export async function getAuthConfig() {
  const { data } = await api.get('/auth/config')
  return data
}

export async function getMe() {
  const { data } = await api.get('/auth/me', {
    // Unauthenticated is normal before login — avoid noisy console 401s.
    validateStatus: (s) => (s >= 200 && s < 300) || s === 401,
  })
  if (!data || data.error === 'unauthorized') {
    const err = new Error('unauthorized')
    err.response = { status: 401, data }
    throw err
  }
  return data
}

export function loginUrl() {
  const base = api.defaults.baseURL || '/api/v1'
  return `${base}/auth/login`
}

export function logoutUrl() {
  const base = api.defaults.baseURL || '/api/v1'
  return `${base}/auth/logout`
}

export async function listProfiles() {
  const { data } = await api.get('/profiles')
  return data
}

export async function getCatalog() {
  const { data } = await api.get('/catalog')
  return data
}

export async function listMockups() {
  const { data } = await api.get('/mockups')
  return data
}

export async function createMockup(payload) {
  const { data } = await api.post('/mockups', payload)
  return data
}

export async function getMockup(id) {
  const { data } = await api.get(`/mockups/${id}`)
  return data
}

export async function saveMockup(id, mockup) {
  const { data } = await api.put(`/mockups/${id}`, mockup)
  return data
}

export async function patchLayout(id, layout) {
  const { data } = await api.patch(`/mockups/${id}/layout`, layout)
  return data
}

export async function addCluster(id) {
  const { data } = await api.post(`/mockups/${id}/clusters`)
  return data
}

export async function deleteCluster(id, clusterId) {
  const { data } = await api.delete(`/mockups/${id}/clusters/${clusterId}`)
  return data
}

export async function deriveMockup(id) {
  const { data } = await api.post(`/mockups/${id}/derive`)
  return data
}

export async function validateMockup(id, mockup) {
  // Body = teaching/topology-only check (no phase advance). Omit body to ValidatePlan + persist.
  if (mockup) {
    const { data } = await api.post(`/mockups/${id}/validate`, mockup)
    return data
  }
  const { data } = await api.post(`/mockups/${id}/validate`)
  return data
}

export async function deployMockup(id) {
  const { data } = await api.post(`/mockups/${id}/deploy`, undefined, { timeout: 90_000 })
  return data
}

export async function listInventory() {
  const { data } = await api.get('/inventory')
  return data
}

export async function createInventory(payload) {
  const { data } = await api.post('/inventory', payload)
  return data
}

export async function getInventory(id) {
  const { data } = await api.get(`/inventory/${id}`)
  return data
}

export async function updateInventory(id, host) {
  const { data } = await api.put(`/inventory/${id}`, host)
  return data
}

export async function probeInventory(id) {
  const { data } = await api.post(`/inventory/${id}/probe`)
  return data
}

export async function fixInventory(id, payload = {}) {
  const { data } = await api.post(`/inventory/${id}/fix`, payload)
  return data
}

export async function deleteInventory(id) {
  await api.delete(`/inventory/${id}`)
}

export async function deleteMockup(id) {
  await api.delete(`/mockups/${id}`)
}

export function imageSetName(version) {
  const compact = String(version || '4.18').replace(/\./g, '')
  return `img${compact}-x86-64-appsub`
}
