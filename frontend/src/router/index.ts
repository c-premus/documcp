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
    },
    {
      path: '/documents',
      name: 'documents',
      component: () => import('@/views/DocumentListView.vue'),
    },
    {
      path: '/documents/trash',
      name: 'documents-trash',
      component: () => import('@/views/DocumentTrashView.vue'),
    },
    {
      path: '/documents/:uuid',
      name: 'document-detail',
      component: () => import('@/views/DocumentDetailView.vue'),
      props: true,
    },
    {
      path: '/users',
      name: 'users',
      component: () => import('@/views/UserListView.vue'),
      meta: { requiresAdmin: true },
    },
    {
      path: '/oauth-clients',
      name: 'oauth-clients',
      component: () => import('@/views/OAuthClientListView.vue'),
      meta: { requiresAdmin: true },
    },
    {
      path: '/external-services',
      name: 'external-services',
      component: () => import('@/views/ExternalServiceListView.vue'),
      meta: { requiresAdmin: true },
    },
    {
      path: '/zim-archives',
      name: 'zim-archives',
      component: () => import('@/views/ZimArchiveListView.vue'),
    },
    {
      path: '/zim-archives/:archive',
      name: 'zim-archive-browse',
      component: () => import('@/views/ZimArchiveBrowseView.vue'),
      props: true,
    },
    {
      path: '/git-templates',
      name: 'git-templates',
      component: () => import('@/views/GitTemplateListView.vue'),
    },
    {
      path: '/git-templates/:uuid/files',
      name: 'git-template-files',
      component: () => import('@/views/GitTemplateFilesView.vue'),
      props: true,
    },
    {
      path: '/queue',
      name: 'queue',
      component: () => import('@/views/QueueView.vue'),
      meta: { requiresAdmin: true },
    },
    {
      path: '/api-docs',
      name: 'api-docs',
      component: () => import('@/views/ApiDocsView.vue'),
    },
  ],
})

router.beforeEach(authGuard)

export { router as default }
