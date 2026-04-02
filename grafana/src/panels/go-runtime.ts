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
  ThresholdsConfigBuilder,
  ThresholdsMode,
} from '@grafana/grafana-foundation-sdk/dashboard';

const PROMETHEUS_DATASOURCE = { type: 'prometheus', uid: 'prometheus' } as const;

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
    .gridPos({ h: 8, w: 8, x: 0, y: 42 })
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
    .gridPos({ h: 8, w: 8, x: 8, y: 42 })
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

export function redisConnectionPoolPanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('Redis Connection Pool')
    .description('Redis connection pool: active, idle, hits, misses, and timeouts')
    .gridPos({ h: 8, w: 8, x: 16, y: 42 })
    .unit('short');

  applyTimeseriesStyle(panel);

  panel
    .withTarget(
      new DataqueryBuilder()
        .refId('A')
        .expr('documcp_redis_active_connections')
        .legendFormat('Active'),
    )
    .withTarget(
      new DataqueryBuilder()
        .refId('B')
        .expr('documcp_redis_idle_connections')
        .legendFormat('Idle'),
    )
    .withTarget(
      new DataqueryBuilder()
        .refId('C')
        .expr('rate(documcp_redis_pool_hits_total[$__rate_interval])')
        .legendFormat('Hits/s'),
    )
    .withTarget(
      new DataqueryBuilder()
        .refId('D')
        .expr('rate(documcp_redis_pool_misses_total[$__rate_interval])')
        .legendFormat('Misses/s'),
    )
    .withTarget(
      new DataqueryBuilder()
        .refId('E')
        .expr('rate(documcp_redis_pool_timeouts_total[$__rate_interval])')
        .legendFormat('Timeouts/s'),
    );

  panel.overrideByName('Active', [
    { id: 'color', value: { fixedColor: 'orange', mode: 'fixed' } },
  ]);
  panel.overrideByName('Idle', [
    { id: 'color', value: { fixedColor: 'green', mode: 'fixed' } },
  ]);
  panel.overrideByName('Hits/s', [
    { id: 'color', value: { fixedColor: 'blue', mode: 'fixed' } },
  ]);
  panel.overrideByName('Misses/s', [
    { id: 'color', value: { fixedColor: 'yellow', mode: 'fixed' } },
  ]);
  panel.overrideByName('Timeouts/s', [
    { id: 'color', value: { fixedColor: 'red', mode: 'fixed' } },
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
    .gridPos({ h: 4, w: 4, x: 0, y: 50 })
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
    .gridPos({ h: 4, w: 4, x: 4, y: 50 })
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
    .gridPos({ h: 8, w: 8, x: 16, y: 50 })
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

export function nativeHttpLatencyPanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('HTTP Latency (Native)')
    .description('P50, P95, P99 request latency from native Prometheus histogram (always sampled, no trace gaps)')
    .gridPos({ h: 4, w: 8, x: 0, y: 54 })
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
        .expr(`histogram_quantile(${q.quantile}, sum(rate(documcp_http_request_duration_seconds_bucket[$__rate_interval])) by (le))`)
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
    .description('Request rate from native Prometheus counter by method')
    .gridPos({ h: 8, w: 8, x: 8, y: 50 })
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

export function queueJobRatePanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('Queue Job Rate')
    .description('Jobs dispatched, completed, and failed per second by type')
    .gridPos({ h: 8, w: 12, x: 0, y: 58 })
    .unit('ops');

  applyTimeseriesStyle(panel);

  panel
    .withTarget(
      new DataqueryBuilder()
        .refId('A')
        .expr('sum by (job_kind) (rate(documcp_queue_jobs_completed_total[$__rate_interval]))')
        .legendFormat('{{job_kind}} completed'),
    )
    .withTarget(
      new DataqueryBuilder()
        .refId('B')
        .expr('sum by (job_kind) (rate(documcp_queue_jobs_failed_total[$__rate_interval]))')
        .legendFormat('{{job_kind}} failed'),
    )
    .withTarget(
      new DataqueryBuilder()
        .refId('C')
        .expr('sum(rate(documcp_queue_jobs_dispatched_total[$__rate_interval]))')
        .legendFormat('dispatched (all)'),
    );

  return panel;
}

export function queueJobDurationPanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('Queue Job Duration (P95)')
    .description('P95 job execution time by type')
    .gridPos({ h: 8, w: 12, x: 12, y: 58 })
    .unit('s');

  applyTimeseriesStyle(panel);

  panel.withTarget(
    new DataqueryBuilder()
      .refId('A')
      .expr(
        `histogram_quantile(0.95, sum by (job_kind, le) (rate(documcp_queue_job_duration_seconds_bucket[$__rate_interval])))`,
      )
      .legendFormat('{{job_kind}}'),
  );

  return panel;
}
