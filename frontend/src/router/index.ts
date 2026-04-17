import { createRouter, createWebHistory } from 'vue-router'
import { authGuard } from '@/auth/authGuard'

const router = createRouter({
  history: createWebHistory('/admin'),
  routes: [
    {
      path: '/',
      redirect: '/dashboard',
    },
    {
      path: '/dashboard',
      name: 'dashboard',
      component: () => import('@/views/DashboardView.vue'),
      meta: { title: 'Dashboard' },
    },
    {
      path: '/documents',
      name: 'documents',
      component: () => import('@/views/DocumentListView.vue'),
      meta: { title: 'Documents' },
    },
    {
      path: '/documents/trash',
      name: 'documents-trash',
      component: () => import('@/views/DocumentTrashView.vue'),
      meta: { title: 'Trash' },
    },
    {
      path: '/documents/:uuid',
      name: 'document-detail',
      component: () => import('@/views/DocumentDetailView.vue'),
      props: true,
      meta: { title: 'Document' },
    },
    {
      path: '/users',
      name: 'users',
      component: () => import('@/views/UserListView.vue'),
      meta: { title: 'Users', requiresAdmin: true },
    },
    {
      path: '/oauth-clients',
      name: 'oauth-clients',
      component: () => import('@/views/OAuthClientListView.vue'),
      meta: { title: 'OAuth Clients', requiresAdmin: true },
    },
    {
      path: '/oauth-clients/:id',
      name: 'oauth-client-detail',
      component: () => import('@/views/OAuthClientDetailView.vue'),
      props: (route) => ({ id: Number(route.params.id) }),
      meta: { title: 'OAuth Client', requiresAdmin: true },
    },
    {
      path: '/external-services',
      name: 'external-services',
      component: () => import('@/views/ExternalServiceListView.vue'),
      meta: { title: 'External Services', requiresAdmin: true },
    },
    {
      path: '/zim-archives',
      name: 'zim-archives',
      component: () => import('@/views/ZimArchiveListView.vue'),
      meta: { title: 'ZIM Archives' },
    },
    {
      path: '/zim-archives/:archive',
      name: 'zim-archive-browse',
      component: () => import('@/views/ZimArchiveBrowseView.vue'),
      props: true,
      meta: { title: 'Browse Archive' },
    },
    {
      path: '/git-templates',
      name: 'git-templates',
      component: () => import('@/views/GitTemplateListView.vue'),
      meta: { title: 'Git Templates' },
    },
    {
      path: '/git-templates/:uuid/files',
      name: 'git-template-files',
      component: () => import('@/views/GitTemplateFilesView.vue'),
      props: true,
      meta: { title: 'Template Files' },
    },
    {
      path: '/queue',
      name: 'queue',
      component: () => import('@/views/QueueView.vue'),
      meta: { title: 'Queue', requiresAdmin: true },
    },
  ],
})

router.beforeEach(authGuard)

router.afterEach((to) => {
  const title = to.meta.title as string | undefined
  document.title = title ? `${title} - DocuMCP` : 'DocuMCP'
})

export { router as default }
