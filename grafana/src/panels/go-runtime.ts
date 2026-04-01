import { PanelBuilder } from '@grafana/grafana-foundation-sdk/timeseries';
import { PanelBuilder as StatPanelBuilder } from '@grafana/grafana-foundation-sdk/stat';
import { DataqueryBuilder } from '@grafana/grafana-foundation-sdk/prometheus';
import {
  BigValueColorMode,
  BigValueGraphMode,
  BigValueTextMode,
  GraphDrawStyle,
  GraphGradientMode,
  LegendDisplayMode,
  LegendPlacement,
  LineInterpolation,
  ReduceDataOptionsBuilder,
  SortOrder,
  TooltipDisplayMode,
  VisibilityMode,
  VizLegendOptionsBuilder,
  VizTooltipOptionsBuilder,
} from '@grafana/grafana-foundation-sdk/common';
import {
  FieldColorBuilder,
  FieldColorModeId,
  ThresholdsConfigBuilder,
  ThresholdsMode,
} from '@grafana/grafana-foundation-sdk/dashboard';

const PROMETHEUS_DATASOURCE = { type: 'prometheus', uid: 'prometheus' } as const;

const LATENCY_BUCKET_METRIC = 'traces_spanmetrics_latency_bucket';
const SERVICE_FILTER = 'service="documcp"';

function buildLegend(): VizLegendOptionsBuilder {
  return new VizLegendOptionsBuilder()
    .displayMode(LegendDisplayMode.List)
    .placement(LegendPlacement.Bottom)
    .showLegend(true);
}

function buildTooltip(): VizTooltipOptionsBuilder {
  return new VizTooltipOptionsBuilder()
    .mode(TooltipDisplayMode.Multi)
    .sort(SortOrder.Descending);
}

function applyTimeseriesStyle(panel: PanelBuilder): PanelBuilder {
  return panel
    .datasource(PROMETHEUS_DATASOURCE)
    .drawStyle(GraphDrawStyle.Line)
    .lineInterpolation(LineInterpolation.Smooth)
    .lineWidth(3)
    .fillOpacity(20)
    .gradientMode(GraphGradientMode.Scheme)
    .showPoints(VisibilityMode.Auto)
    .pointSize(4)
    .legend(buildLegend())
    .tooltip(buildTooltip());
}

// --- Database Connection Pool ---

export function dbConnectionPoolPanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('DB Connection Pool')
    .description('PostgreSQL connection pool: open, in-use, and idle connections')
    .gridPos({ h: 8, w: 8, x: 0, y: 34 })
    .unit('short');

  applyTimeseriesStyle(panel);

  panel
    .withTarget(
      new DataqueryBuilder()
        .refId('A')
        .expr('documcp_db_open_connections')
        .legendFormat('Open'),
    )
    .withTarget(
      new DataqueryBuilder()
        .refId('B')
        .expr('documcp_db_in_use_connections')
        .legendFormat('In Use'),
    )
    .withTarget(
      new DataqueryBuilder()
        .refId('C')
        .expr('documcp_db_idle_connections')
        .legendFormat('Idle'),
    );

  panel.overrideByName('Open', [
    { id: 'color', value: { fixedColor: 'blue', mode: 'fixed' } },
  ]);
  panel.overrideByName('In Use', [
    { id: 'color', value: { fixedColor: 'orange', mode: 'fixed' } },
  ]);
  panel.overrideByName('Idle', [
    { id: 'color', value: { fixedColor: 'green', mode: 'fixed' } },
  ]);

  return panel;
}

export function dbWaitPanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('DB Connection Wait')
    .description('Rate of connections waited for and cumulative wait time')
    .gridPos({ h: 8, w: 8, x: 8, y: 34 })
    .unit('short');

  applyTimeseriesStyle(panel);

  panel
    .withTarget(
      new DataqueryBuilder()
        .refId('A')
        .expr('rate(documcp_db_wait_count_total[$__rate_interval])')
        .legendFormat('Wait rate (/s)'),
    )
    .withTarget(
      new DataqueryBuilder()
        .refId('B')
        .expr('rate(documcp_db_wait_duration_seconds_total[$__rate_interval])')
        .legendFormat('Wait time (s/s)'),
    );

  panel.overrideByName('Wait rate (/s)', [
    { id: 'color', value: { fixedColor: 'orange', mode: 'fixed' } },
  ]);
  panel.overrideByName('Wait time (s/s)', [
    { id: 'color', value: { fixedColor: 'red', mode: 'fixed' } },
    { id: 'unit', value: 's' },
  ]);

  return panel;
}

// --- HTTP & Application Metrics ---

export function activeConnectionsPanel(): StatPanelBuilder {
  const query = new DataqueryBuilder()
    .refId('A')
    .expr('documcp_http_active_connections')
    .legendFormat('Active');

  return new StatPanelBuilder()
    .title('Active Connections')
    .description('Currently active HTTP connections')
    .gridPos({ h: 4, w: 4, x: 16, y: 34 })
    .datasource(PROMETHEUS_DATASOURCE)
    .withTarget(query)
    .unit('short')
    .thresholds(
      new ThresholdsConfigBuilder()
        .mode(ThresholdsMode.Absolute)
        .steps([
          { color: 'green', value: null },
          { color: 'yellow', value: 50 },
          { color: 'red', value: 100 },
        ]),
    )
    .colorMode(BigValueColorMode.Background)
    .graphMode(BigValueGraphMode.Area)
    .textMode(BigValueTextMode.ValueAndName)
    .reduceOptions(
      new ReduceDataOptionsBuilder().calcs(['lastNotNull']),
    );
}

export function documentCountPanel(): StatPanelBuilder {
  const query = new DataqueryBuilder()
    .refId('A')
    .expr('documcp_documents')
    .legendFormat('Documents');

  return new StatPanelBuilder()
    .title('Document Count')
    .description('Total documents managed')
    .gridPos({ h: 4, w: 4, x: 20, y: 34 })
    .datasource(PROMETHEUS_DATASOURCE)
    .withTarget(query)
    .unit('short')
    .thresholds(
      new ThresholdsConfigBuilder()
        .mode(ThresholdsMode.Absolute)
        .steps([
          { color: 'blue', value: null },
        ]),
    )
    .colorMode(BigValueColorMode.Background)
    .graphMode(BigValueGraphMode.Area)
    .textMode(BigValueTextMode.ValueAndName)
    .reduceOptions(
      new ReduceDataOptionsBuilder().calcs(['lastNotNull']),
    );
}

// --- Search & Native HTTP Metrics ---

export function searchLatencyPanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('Search Latency')
    .description('P50, P95, P99 search query latency')
    .gridPos({ h: 8, w: 8, x: 16, y: 38 })
    .unit('s');

  applyTimeseriesStyle(panel);

  const quantiles: ReadonlyArray<{ refId: string; quantile: string; legend: string; color: string }> = [
    { refId: 'A', quantile: '0.50', legend: 'P50', color: 'green' },
    { refId: 'B', quantile: '0.95', legend: 'P95', color: 'orange' },
    { refId: 'C', quantile: '0.99', legend: 'P99', color: 'red' },
  ];

  for (const q of quantiles) {
    panel.withTarget(
      new DataqueryBuilder()
        .refId(q.refId)
        .expr(`histogram_quantile(${q.quantile}, sum(rate(documcp_search_latency_seconds_bucket[$__rate_interval])) by (le))`)
        .legendFormat(q.legend),
    );

    panel.overrideByName(q.legend, [
      { id: 'color', value: { fixedColor: q.color, mode: 'fixed' } },
    ]);
  }

  return panel;
}

export function nativeHttpRatePanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('HTTP Requests (Native)')
    .description('Request rate from native Prometheus counter by method and route')
    .gridPos({ h: 8, w: 12, x: 0, y: 38 })
    .unit('reqps');

  applyTimeseriesStyle(panel);

  panel.withTarget(
    new DataqueryBuilder()
      .refId('A')
      .expr('sum by (method) (rate(documcp_http_requests_total[$__rate_interval]))')
      .legendFormat('{{method}}'),
  );

  return panel;
}

export function mcpKiwixPanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('MCP & Kiwix Operations')
    .description('MCP tool and Kiwix service operation latency (P95)')
    .gridPos({ h: 8, w: 12, x: 12, y: 38 })
    .unit('s');

  applyTimeseriesStyle(panel);

  panel.withTarget(
    new DataqueryBuilder()
      .refId('A')
      .expr(
        `histogram_quantile(0.95, sum by (span_name, le) (rate(${LATENCY_BUCKET_METRIC}{${SERVICE_FILTER}, span_name=~"mcp.*|kiwix.*|action.zim.*"}[$__rate_interval])))`,
      )
      .legendFormat('{{span_name}}'),
  );

  return panel;
}
