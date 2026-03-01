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
  httpCallsPanel,
} from '../panels/dependencies.js';
import {
  dbConnectionPoolPanel,
  dbWaitPanel,
  activeConnectionsPanel,
  documentCountPanel,
  searchLatencyPanel,
  nativeHttpRatePanel,
  mcpKiwixPanel,
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

    // Section 3: Dependencies (SQL, HTTP)
    .withRow(new RowBuilder('Dependencies (SQL, HTTP)'))
    .withPanel(sqlRatePanel())
    .withPanel(sqlLatencyPanel())
    .withPanel(httpCallsPanel())

    // Section 4: Go Runtime & Application Metrics
    .withRow(new RowBuilder('Go Runtime & Application Metrics'))
    .withPanel(dbConnectionPoolPanel())
    .withPanel(dbWaitPanel())
    .withPanel(activeConnectionsPanel())
    .withPanel(documentCountPanel())
    .withPanel(nativeHttpRatePanel())
    .withPanel(mcpKiwixPanel())
    .withPanel(searchLatencyPanel())

    // Section 5: Cross-Service Topology
    .withRow(new RowBuilder('Cross-Service Topology'))
    .withPanel(hopLatencyPanel())
    .withPanel(edgeRequestRatePanel())
    .withPanel(edgeErrorRatePanel())

    // Section 6: Trace Explorer
    .withRow(new RowBuilder('Trace Explorer'))
    .withPanel(recentTracesPanel())
    .withPanel(serviceMapPanel())

    // Section 7: Logs
    .withRow(new RowBuilder('Logs'))
    .withPanel(logVolumePanel())
    .withPanel(recentLogsPanel());
}
