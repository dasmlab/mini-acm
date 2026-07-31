import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    component: () => import('src/layouts/MainLayout.vue'),
    children: [
      { path: '', name: 'home', component: () => import('src/pages/HomePage.vue') },
      { path: 'mockups', name: 'mockups', component: () => import('src/pages/MockUpsPage.vue') },
      {
        path: 'mockups/:id/topology',
        name: 'topology',
        component: () => import('src/pages/TopologyPage.vue'),
        props: true,
      },
      {
        path: 'mockups/:id/wizard',
        name: 'wizard',
        component: () => import('src/pages/WizardPage.vue'),
        props: true,
      },
    ],
  },
]

export default createRouter({
  history: createWebHistory(),
  routes,
})
