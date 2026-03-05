import { useSSE } from '@/composables/useSSE'
import { useDocumentsStore } from '@/stores/documents'
import { useNotificationsStore } from '@/stores/notifications'
import { toast } from 'vue-sonner'

const schedulerMessages: Record<string, string> = {
  'sync_kiwix': 'Kiwix sync completed',
  'sync_confluence': 'Confluence sync completed',
  'sync_git_templates': 'Git template sync completed',
  'cleanup_oauth_tokens': 'OAuth token cleanup completed',
  'health_check_services': 'Service health checks completed',
  'cleanup_orphaned_files': 'Orphaned files cleanup completed',
  'verify_search_indexes': 'Search index verification completed',
  'purge_soft_deleted': 'Soft-deleted records purged',
  'cleanup_zim_archives': 'ZIM archive cleanup completed',
}

export function useDocumentEvents() {
  const { connect, on, connected } = useSSE()
  const documents = useDocumentsStore()
  const notifications = useNotificationsStore()

  function start() {
    connect()

    on('job.completed', (event) => {
      notifications.addEvent(event)

      if (event.job_kind === 'document_extract') {
        toast.success('Document extracted successfully')
        documents.refreshCurrent()
      }
      if (event.job_kind === 'document_index') {
        toast.success('Document indexed successfully')
        documents.refreshCurrent()
      }

      const schedulerMsg = schedulerMessages[event.job_kind]
      if (schedulerMsg) {
        toast.info(schedulerMsg)
      }
    })

    on('job.failed', (event) => {
      notifications.addEvent(event)

      if (event.job_kind.startsWith('document_')) {
        toast.error(`Document processing failed: ${event.error ?? 'Unknown error'}`)
      }
    })

    on('job.snoozed', (event) => {
      notifications.addEvent(event)
    })
  }

  return { start, connected }
}
