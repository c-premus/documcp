import { PanelBuilder } from '@grafana/grafana-foundation-sdk/timeseries';
import { DataqueryBuilder } from '@grafana/grafana-foundation-sdk/prometheus';
import {
  GraphDrawStyle,
  GraphGradientMode,
  LegendDisplayMode,
  LegendPlacement,
  LineInterpolation,
  SortOrder,
  TooltipDisplayMode,
  VisibilityMode,
  VizLegendOptionsBuilder,
  VizTooltipOptionsBuilder,
} from '@grafana/grafana-foundation-sdk/common';

const PROMETHEUS_DATASOURCE = { type: 'prometheus', uid: 'prometheus' } as const;

const SQL_OPERATIONS = 'SELECT|INSERT|UPDATE|DELETE';
const REDIS_COMMANDS = '[a-z]+';
const GIT_OPERATIONS = 'git\\\\..*';

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

function applyCommonStyle(panel: PanelBuilder, fillOpacity: number): PanelBuilder {
  return panel
    .datasource(PROMETHEUS_DATASOURCE)
    .drawStyle(GraphDrawStyle.Line)
    .lineInterpolation(LineInterpolation.Smooth)
    .lineWidth(3)
    .fillOpacity(fillOpacity)
    .gradientMode(GraphGradientMode.Scheme)
    .showPoints(VisibilityMode.Auto)
    .pointSize(4)
    .legend(buildLegend())
    .tooltip(buildTooltip());
}

export function sqlRatePanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('SQL Query Rate')
    .description('SQL operations per second by type')
    .gridPos({ h: 8, w: 8, x: 0, y: 25 })
    .unit('ops');

  applyCommonStyle(panel, 25);

  const query = new DataqueryBuilder()
    .refId('A')
    .expr(`sum by (span_name) (rate(traces_spanmetrics_calls_total{service="documcp", span_name=~"${SQL_OPERATIONS}"}[$__rate_interval]))`)
    .legendFormat('{{span_name}}');

  return panel.withTarget(query);
}

export function sqlLatencyPanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('SQL Latency (P95)')
    .description('P95 latency for SQL queries by type')
    .gridPos({ h: 8, w: 8, x: 8, y: 25 })
    .unit('s');

  applyCommonStyle(panel, 20);

  const query = new DataqueryBuilder()
    .refId('A')
    .expr(`histogram_quantile(0.95, sum by (span_name, le) (rate(traces_spanmetrics_latency_bucket{service="documcp", span_name=~"${SQL_OPERATIONS}"}[$__rate_interval])))`)
    .legendFormat('{{span_name}}');

  return panel.withTarget(query);
}

export function redisCommandRatePanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('Redis Command Rate')
    .description('Redis operations per second by command')
    .gridPos({ h: 8, w: 8, x: 16, y: 25 })
    .unit('ops');

  applyCommonStyle(panel, 25);

  const query = new DataqueryBuilder()
    .refId('A')
    .expr(`sum by (span_name) (rate(traces_spanmetrics_calls_total{service="documcp", span_kind="SPAN_KIND_CLIENT", span_name=~"${REDIS_COMMANDS}"}[$__rate_interval]))`)
    .legendFormat('{{span_name}}');

  return panel.withTarget(query);
}

export function externalCallsPanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('External Service Calls')
    .description('Outbound HTTP (Kiwix) and Git operation rates')
    .gridPos({ h: 8, w: 8, x: 0, y: 33 })
    .unit('ops');

  applyCommonStyle(panel, 25);

  const httpQuery = new DataqueryBuilder()
    .refId('A')
    .expr('sum(rate(traces_spanmetrics_calls_total{service="documcp", span_kind="SPAN_KIND_CLIENT", span_name=~"HTTP .*"}[$__rate_interval]))')
    .legendFormat('HTTP (Kiwix)');

  const gitQuery = new DataqueryBuilder()
    .refId('B')
    .expr(`sum(rate(traces_spanmetrics_calls_total{service="documcp", span_kind="SPAN_KIND_CLIENT", span_name=~"${GIT_OPERATIONS}"}[$__rate_interval]))`)
    .legendFormat('Git ops');

  return panel.withTarget(httpQuery).withTarget(gitQuery);
}

export function redisCommandLatencyPanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('Redis Latency (P95)')
    .description('P95 latency for Redis commands')
    .gridPos({ h: 8, w: 8, x: 8, y: 33 })
    .unit('s');

  applyCommonStyle(panel, 20);

  const query = new DataqueryBuilder()
    .refId('A')
    .expr(`histogram_quantile(0.95, sum by (span_name, le) (rate(traces_spanmetrics_latency_bucket{service="documcp", span_kind="SPAN_KIND_CLIENT", span_name=~"${REDIS_COMMANDS}"}[$__rate_interval])))`)
    .legendFormat('{{span_name}}');

  return panel.withTarget(query);
}

export function gitOperationLatencyPanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('Git & HTTP Latency (P95)')
    .description('P95 latency for Git operations and outbound HTTP calls')
    .gridPos({ h: 8, w: 8, x: 16, y: 33 })
    .unit('s');

  applyCommonStyle(panel, 20);

  const gitQuery = new DataqueryBuilder()
    .refId('A')
    .expr(`histogram_quantile(0.95, sum by (span_name, le) (rate(traces_spanmetrics_latency_bucket{service="documcp", span_kind="SPAN_KIND_CLIENT", span_name=~"${GIT_OPERATIONS}"}[$__rate_interval])))`)
    .legendFormat('{{span_name}}');

  const httpQuery = new DataqueryBuilder()
    .refId('B')
    .expr('histogram_quantile(0.95, sum by (le) (rate(traces_spanmetrics_latency_bucket{service="documcp", span_kind="SPAN_KIND_CLIENT", span_name=~"HTTP .*"}[$__rate_interval])))')
    .legendFormat('HTTP (Kiwix)');

  return panel.withTarget(gitQuery).withTarget(httpQuery);
}
