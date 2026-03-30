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
import {
  FieldColorBuilder,
  FieldColorModeId,
} from '@grafana/grafana-foundation-sdk/dashboard';

const PROMETHEUS_DATASOURCE = { type: 'prometheus', uid: 'prometheus' } as const;

const CALLS_METRIC = 'traces_spanmetrics_calls_total';
const LATENCY_METRIC = 'traces_spanmetrics_latency_bucket';
const SPAN_FILTER = 'service="DocuMCP", span_kind=~"SPAN_KIND_SERVER|SPAN_KIND_INTERNAL"';

function defaultLegend(): VizLegendOptionsBuilder {
  return new VizLegendOptionsBuilder()
    .displayMode(LegendDisplayMode.List)
    .placement(LegendPlacement.Bottom);
}

function defaultTooltip(): VizTooltipOptionsBuilder {
  return new VizTooltipOptionsBuilder()
    .mode(TooltipDisplayMode.Multi)
    .sort(SortOrder.Descending);
}

function paletteClassicColor(): FieldColorBuilder {
  return new FieldColorBuilder().mode(FieldColorModeId.PaletteClassic);
}

function fixedColor(color: string): FieldColorBuilder {
  return new FieldColorBuilder().mode(FieldColorModeId.Fixed).fixedColor(color);
}

export function requestRatePanel(): PanelBuilder {
  const query = new DataqueryBuilder()
    .refId('A')
    .expr(`sum(rate(${CALLS_METRIC}{${SPAN_FILTER}}[$__rate_interval]))`)
    .legendFormat('Total');

  return new PanelBuilder()
    .title('Request Rate')
    .description('Requests per second (server + internal spans)')
    .gridPos({ h: 8, w: 8, x: 0, y: 1 })
    .datasource(PROMETHEUS_DATASOURCE)
    .withTarget(query)
    .unit('reqps')
    .colorScheme(paletteClassicColor())
    .drawStyle(GraphDrawStyle.Line)
    .lineInterpolation(LineInterpolation.Smooth)
    .lineWidth(3)
    .fillOpacity(30)
    .gradientMode(GraphGradientMode.Scheme)
    .showPoints(VisibilityMode.Auto)
    .pointSize(4)
    .axisBorderShow(false)
    .legend(defaultLegend())
    .tooltip(defaultTooltip());
}

export function errorRatePanel(): PanelBuilder {
  const query = new DataqueryBuilder()
    .refId('A')
    .expr(`sum(rate(${CALLS_METRIC}{${SPAN_FILTER}, status_code="STATUS_CODE_ERROR"}[$__rate_interval]))`)
    .legendFormat('Errors');

  return new PanelBuilder()
    .title('Error Rate')
    .description('Server errors per second')
    .gridPos({ h: 8, w: 8, x: 8, y: 1 })
    .datasource(PROMETHEUS_DATASOURCE)
    .withTarget(query)
    .unit('reqps')
    .colorScheme(fixedColor('red'))
    .drawStyle(GraphDrawStyle.Line)
    .lineInterpolation(LineInterpolation.Smooth)
    .lineWidth(3)
    .fillOpacity(30)
    .gradientMode(GraphGradientMode.Scheme)
    .showPoints(VisibilityMode.Auto)
    .pointSize(4)
    .axisBorderShow(false)
    .legend(defaultLegend())
    .tooltip(defaultTooltip());
}

export function requestLatencyPanel(): PanelBuilder {
  const quantiles: ReadonlyArray<{ refId: string; quantile: string; legend: string; color: string }> = [
    { refId: 'A', quantile: '0.50', legend: 'P50', color: 'green' },
    { refId: 'B', quantile: '0.95', legend: 'P95', color: 'orange' },
    { refId: 'C', quantile: '0.99', legend: 'P99', color: 'red' },
  ];

  const panel = new PanelBuilder()
    .title('Request Latency')
    .description('P50, P95, P99 latency for server spans')
    .gridPos({ h: 8, w: 8, x: 16, y: 1 })
    .datasource(PROMETHEUS_DATASOURCE)
    .unit('s')
    .colorScheme(paletteClassicColor())
    .drawStyle(GraphDrawStyle.Line)
    .lineInterpolation(LineInterpolation.Smooth)
    .lineWidth(3)
    .fillOpacity(20)
    .gradientMode(GraphGradientMode.Scheme)
    .showPoints(VisibilityMode.Auto)
    .pointSize(4)
    .axisBorderShow(false)
    .legend(defaultLegend())
    .tooltip(defaultTooltip());

  for (const q of quantiles) {
    panel.withTarget(
      new DataqueryBuilder()
        .refId(q.refId)
        .expr(`histogram_quantile(${q.quantile}, sum(rate(${LATENCY_METRIC}{${SPAN_FILTER}}[$__rate_interval])) by (le))`)
        .legendFormat(q.legend),
    );

    panel.overrideByName(q.legend, [
      { id: 'color', value: { fixedColor: q.color, mode: 'fixed' } },
    ]);
  }

  return panel;
}
