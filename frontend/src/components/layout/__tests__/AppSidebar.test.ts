import { describe, it, expect, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createWebHistory } from 'vue-router'
import AppSidebar from '@/components/layout/AppSidebar.vue'
import { useAuthStore } from '@/stores/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [{ path: '/:pathMatch(.*)*', name: 'catchAll', component: { template: '<div />' } }],
})

const standardNavItems = ['Dashboard', 'Documents', 'ZIM Archives', 'Git Templates']
const adminNavItems = ['Users', 'OAuth Clients', 'External Services', 'Queue']

describe('AppSidebar', () => {
  beforeEach(async () => {
    await router.push('/')
    await router.isReady()
  })

  it('renders all 4 standard nav items', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const auth = useAuthStore()
    auth.$patch({
      user: { id: 1, email: 'test@example.com', name: 'Test', is_admin: false },
      loading: false,
    })

    const wrapper = mount(AppSidebar, {
      global: { plugins: [pinia, router] },
    })

    for (const name of standardNavItems) {
      expect(wrapper.text()).toContain(name)
    }
  })

  it('does not show admin items when user is not admin', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const auth = useAuthStore()
    auth.$patch({
      user: { id: 1, email: 'test@example.com', name: 'Test', is_admin: false },
      loading: false,
    })

    const wrapper = mount(AppSidebar, {
      global: { plugins: [pinia, router] },
    })

    for (const name of adminNavItems) {
      expect(wrapper.text()).not.toContain(name)
    }
  })

  it('shows admin items when user is admin', () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    const auth = useAuthStore()
    auth.$patch({
      user: { id: 1, email: 'admin@example.com', name: 'Admin', is_admin: true },
      loading: false,
    })

    const wrapper = mount(AppSidebar, {
      global: { plugins: [pinia, router] },
    })

    for (const name of adminNavItems) {
      expect(wrapper.text()).toContain(name)
    }
  })
})
