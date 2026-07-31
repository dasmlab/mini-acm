import axios from 'axios'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1',
  timeout: 30_000,
})

export default api

export async function getHealth() {
  const { data } = await api.get('/health')
  return data
}

export async function listProfiles() {
  const { data } = await api.get('/profiles')
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

export async function deriveMockup(id) {
  const { data } = await api.post(`/mockups/${id}/derive`)
  return data
}

export async function deleteMockup(id) {
  await api.delete(`/mockups/${id}`)
}
