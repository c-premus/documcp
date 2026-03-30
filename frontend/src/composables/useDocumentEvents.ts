import { onUnmounted } from 'vue'
import { useSSEStore } from '@/stores/sse'
import { useDocumentsStore } from '@/stores/documents'
import { useZimArchivesStore } from '@/stores/zimArchives'
import { useGitTemplatesStore } from '@/stores/gitTemplates'
import { useExternalServicesStore } from '@/stores/externalServices'
import { useNotificationsStore } from '@/stores/notifications'
import { useQueueStore } from '@/stores/queue'
import { toast } from 'vue-sonner'

const schedulerMessages: Record<string, string> = {
  sync_kiwix: 'Kiwix sync completed',
  sync_git_templates: 'Git template sync completed',
  cleanup_oauth_tokens: 'OAuth token cleanup completed',
  health_check_services: 'Service health checks completed',
  cleanup_orphaned_files: 'Orphaned files cleanup completed',
  purge_soft_deleted: 'Soft-deleted records purged',
}

export function useDocumentEvents() {
  const sseStore = useSSEStore()
  const documents = useDocumentsStore()
  const zimArchives = useZimArchivesStore()
  const gitTemplates = useGitTemplatesStore()
  const externalServices = useExternalServicesStore()
  const notifications = useNotificationsStore()
  const queue = useQueueStore()
  const cleanups: Array<() => void> = []

  function start() {
    cleanups.push(
      sseStore.on('job.completed', (event) => {
        notifications.addEvent(event)

        if (event.job_kind === 'document_extract') {
          toast.success('Document extracted successfully')
          documents.refreshCurrent()
          if (documents.loaded) {
            documents.fetchDocuments()
          }
        }

        if (event.job_kind === 'sync_kiwix') {
          if (zimArchives.loaded) {
            zimArchives.fetchArchives()
          }
          if (externalServices.loaded) {
            externalServices.fetchServices()
          }
        }

        if (event.job_kind === 'sync_git_templates') {
          if (gitTemplates.loaded) {
            gitTemplates.fetchTemplates()
          }
        }

        if (event.job_kind === 'health_check_services' && externalServices.loaded) {
          externalServices.fetchServices()
        }

        if (queue.stats !== null) {
          queue.fetchStats()
        }

        const schedulerMsg = schedulerMessages[event.job_kind]
        if (schedulerMsg) {
          toast.info(schedulerMsg)
        }
      }),
    )

    cleanups.push(
      sseStore.on('job.failed', (event) => {
        notifications.addEvent(event)

        if (event.job_kind.startsWith('document_')) {
          toast.error(`Document processing failed: ${event.error ?? 'Unknown error'}`)
          if (documents.loaded) {
            documents.fetchDocuments()
          }
        }

        if (queue.stats !== null) {
          queue.fetchStats()
          queue.fetchFailedJobs()
        }
      }),
    )

    cleanups.push(
      sseStore.on('job.snoozed', (event) => {
        notifications.addEvent(event)
      }),
    )
  }

  onUnmounted(() => {
    cleanups.forEach((cleanup) => cleanup())
  })

  return { start, connected: sseStore.connected }
}
