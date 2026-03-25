import { onUnmounted } from 'vue'
import { useSSEStore } from '@/stores/sse'
import { useDocumentsStore } from '@/stores/documents'
import { useZimArchivesStore } from '@/stores/zimArchives'
import { useGitTemplatesStore } from '@/stores/gitTemplates'
import { useExternalServicesStore } from '@/stores/externalServices'
import { useNotificationsStore } from '@/stores/notifications'
import { toast } from 'vue-sonner'

const schedulerMessages: Record<string, string> = {
  sync_kiwix: 'Kiwix sync completed',
  sync_git_templates: 'Git template sync completed',
  cleanup_oauth_tokens: 'OAuth token cleanup completed',
  health_check_services: 'Service health checks completed',
  cleanup_orphaned_files: 'Orphaned files cleanup completed',
  verify_search_indexes: 'Search index verification completed',
  purge_soft_deleted: 'Soft-deleted records purged',
  cleanup_zim_archives: 'ZIM archive cleanup completed',
}

export function useDocumentEvents() {
  const sseStore = useSSEStore()
  const documents = useDocumentsStore()
  const zimArchives = useZimArchivesStore()
  const gitTemplates = useGitTemplatesStore()
  const externalServices = useExternalServicesStore()
  const notifications = useNotificationsStore()
  const cleanups: Array<() => void> = []

  function start() {
    cleanups.push(
      sseStore.on('job.completed', (event) => {
        notifications.addEvent(event)

        if (event.job_kind === 'document_extract' || event.job_kind === 'document_index') {
          toast.success(
            event.job_kind === 'document_extract'
              ? 'Document extracted successfully'
              : 'Document indexed successfully',
          )
          documents.refreshCurrent()
          if (documents.documents.length > 0) {
            documents.fetchDocuments()
          }
        }

        if (event.job_kind === 'sync_kiwix' && zimArchives.archives.length > 0) {
          zimArchives.fetchArchives()
        }

        if (event.job_kind === 'sync_git_templates' && gitTemplates.templates.length > 0) {
          gitTemplates.fetchTemplates()
        }

        if (event.job_kind === 'health_check_services' && externalServices.services.length > 0) {
          externalServices.fetchServices()
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
          if (documents.documents.length > 0) {
            documents.fetchDocuments()
          }
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
