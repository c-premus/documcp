import {
  DashboardBuilder,
  DashboardLinkBuilder,
  DashboardLinkType,
  AnnotationQueryBuilder,
  RowBuilder,
} from '@grafana/grafana-foundation-sdk/dashboard';

import {
  requestRatePanel,
  errorRatePanel,
  requestLatencyPanel,
} from '../panels/red-metrics.js';
import { routesTablePanel, slowestRoutesPanel } from '../panels/routes.js';
import {
  sqlRatePanel,
  sqlLatencyPanel,
  redisCommandRatePanel,
  externalCallsPanel,
  redisCommandLatencyPanel,
  gitOperationLatencyPanel,
} from '../panels/dependencies.js';
import {
  dbConnectionPoolPanel,
  dbWaitPanel,
  redisConnectionPoolPanel,
  activeConnectionsPanel,
  documentCountPanel,
  searchLatencyPanel,
  nativeHttpRatePanel,
  queueJobRatePanel,
  queueJobDurationPanel,
} from '../panels/go-runtime.js';
import {
  hopLatencyPanel,
  edgeRequestRatePanel,
  edgeErrorRatePanel,
} from '../panels/cross-service.js';
import { logVolumePanel, recentLogsPanel } from '../panels/logs.js';
import { recentTracesPanel, serviceMapPanel } from '../panels/traces.js';

export function buildDocuMCPDashboard(): DashboardBuilder {
  return new DashboardBuilder('DocuMCP Observability')
    .uid('documcp-observability-v2')
    .description(
      'Traces, logs, and RED metrics for the DocuMCP Go stack (database, search, worker pool)',
    )
    .tags(['documcp', 'observability', 'traces', 'go'])
    .timezone('browser')
    .refresh('30s')
    .time({ from: 'now-1h', to: 'now' })
    .fiscalYearStartMonth(0)
    .liveNow(false)
    .editable()

    .link(
      new DashboardLinkBuilder('Explore Tempo')
        .url('/explore?orgId=1&left=%7B%22datasource%22:%22tempo%22%7D')
        .icon('bolt')
        .type(DashboardLinkType.Link)
        .tooltip('Open Tempo in Explore mode'),
    )
    .link(
      new DashboardLinkBuilder('Explore Loki')
        .url('/explore?orgId=1&left=%7B%22datasource%22:%22loki%22%7D')
        .icon('list-ul')
        .type(DashboardLinkType.Link)
        .tooltip('Open Loki in Explore mode'),
    )

    .annotation(
      new AnnotationQueryBuilder()
        .builtIn(1)
        .datasource({ type: 'grafana', uid: '-- Grafana --' })
        .enable(true)
        .hide(true)
        .iconColor('rgba(0, 211, 255, 1)')
        .name('Annotations & Alerts')
        .type('dashboard'),
    )

    // Section 1: DocuMCP Service Overview (RED Metrics)
    .withRow(new RowBuilder('DocuMCP Service Overview (RED Metrics)'))
    .withPanel(requestRatePanel())
    .withPanel(errorRatePanel())
    .withPanel(requestLatencyPanel())

    // Section 2: Top Routes & Operations
    .withRow(new RowBuilder('Top Routes & Operations'))
    .withPanel(routesTablePanel())
    .withPanel(slowestRoutesPanel())

    // Section 3: Dependencies (SQL, Redis, HTTP, Git)
    .withRow(new RowBuilder('Dependencies (SQL, Redis, HTTP, Git)'))
    .withPanel(sqlRatePanel())
    .withPanel(sqlLatencyPanel())
    .withPanel(redisCommandRatePanel())
    .withPanel(externalCallsPanel())
    .withPanel(redisCommandLatencyPanel())
    .withPanel(gitOperationLatencyPanel())

    // Section 4: Connection Pools & Application Metrics
    .withRow(new RowBuilder('Connection Pools & Application Metrics'))
    .withPanel(dbConnectionPoolPanel())
    .withPanel(dbWaitPanel())
    .withPanel(redisConnectionPoolPanel())
    .withPanel(activeConnectionsPanel())
    .withPanel(documentCountPanel())
    .withPanel(nativeHttpRatePanel())
    .withPanel(searchLatencyPanel())

    // Section 5: Queue Operations
    .withRow(new RowBuilder('Queue Operations'))
    .withPanel(queueJobRatePanel())
    .withPanel(queueJobDurationPanel())

    // Section 6: Cross-Service Topology
    .withRow(new RowBuilder('Cross-Service Topology'))
    .withPanel(hopLatencyPanel())
    .withPanel(edgeRequestRatePanel())
    .withPanel(edgeErrorRatePanel())

    // Section 7: Trace Explorer
    .withRow(new RowBuilder('Trace Explorer'))
    .withPanel(recentTracesPanel())
    .withPanel(serviceMapPanel())

    // Section 8: Logs
    .withRow(new RowBuilder('Logs'))
    .withPanel(logVolumePanel())
    .withPanel(recentLogsPanel());
}
