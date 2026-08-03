import { createRouter, createWebHistory } from 'vue-router'
import { useAuth } from 'src/services/auth'

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('src/pages/LoginPage.vue'),
    meta: { public: true },
  },
  {
    path: '/',
    component: () => import('src/layouts/MainLayout.vue'),
    children: [
      { path: '', name: 'home', component: () => import('src/pages/HomePage.vue'), meta: { admin: true } },
      { path: 'mockups', name: 'mockups', component: () => import('src/pages/MockUpsPage.vue'), meta: { admin: true } },
      { path: 'model', name: 'model', component: () => import('src/pages/ModelPage.vue'), meta: { admin: true } },
      { path: 'inventory', name: 'inventory', component: () => import('src/pages/InventoryPage.vue'), meta: { admin: true } },
      {
        path: 'mockups/:id/topology',
        name: 'topology',
        component: () => import('src/pages/TopologyPage.vue'),
        props: true,
        meta: { admin: true },
      },
      {
        path: 'mockups/:id/wizard',
        name: 'wizard',
        component: () => import('src/pages/WizardPage.vue'),
        props: true,
        meta: { admin: true },
      },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach(async (to) => {
  const auth = useAuth()
  await auth.init()
  if (to.meta.public) return true
  if (!auth.authEnabled.value) return true
  if (!auth.isAuthenticated.value) {
    return { name: 'login', query: { returnTo: to.fullPath } }
  }
  if (to.meta.admin && !auth.isAdmin.value) {
    return { name: 'login', query: { returnTo: to.fullPath } }
  }
  return true
})

export default router
