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

const SG_SERVER_LATENCY = 'traces_service_graph_request_server_seconds_bucket';
const SG_REQUEST_TOTAL = 'traces_service_graph_request_total';
const SG_FAILED_TOTAL = 'traces_service_graph_request_failed_total';
const CLIENT_FILTER = 'client=~"DocuMCP|documcp"';
const SERVER_FILTER = 'server=~"DocuMCP|documcp"';

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

export function hopLatencyPanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('Hop Latency (P95)')
    .description('P95 server-side latency for each hop in the request chain')
    .gridPos({ h: 8, w: 8, x: 0, y: 76 })
    .unit('s');

  applyCommonStyle(panel, 25);

  panel.withTarget(
    new DataqueryBuilder()
      .refId('A')
      .expr(
        `histogram_quantile(0.95, sum by (client, server, le) (rate(${SG_SERVER_LATENCY}{${CLIENT_FILTER}}[$__rate_interval]) or rate(${SG_SERVER_LATENCY}{${SERVER_FILTER}}[$__rate_interval])))`,
      )
      .legendFormat('{{client}} → {{server}}'),
  );

  return panel;
}

export function edgeRequestRatePanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('Edge Request Rate')
    .description('Request rate for each service-to-service edge in the topology')
    .gridPos({ h: 8, w: 8, x: 8, y: 76 })
    .unit('reqps');

  applyCommonStyle(panel, 25);

  panel.withTarget(
    new DataqueryBuilder()
      .refId('A')
      .expr(
        `sum by (client, server) (rate(${SG_REQUEST_TOTAL}{${CLIENT_FILTER}}[$__rate_interval]) or rate(${SG_REQUEST_TOTAL}{${SERVER_FILTER}}[$__rate_interval]))`,
      )
      .legendFormat('{{client}} → {{server}}'),
  );

  return panel;
}

export function edgeErrorRatePanel(): PanelBuilder {
  const panel = new PanelBuilder()
    .title('Edge Error Rate')
    .description('Failed requests per second for each service edge')
    .gridPos({ h: 8, w: 8, x: 16, y: 76 })
    .unit('reqps');

  applyCommonStyle(panel, 25);

  panel.withTarget(
    new DataqueryBuilder()
      .refId('A')
      .expr(
        `sum by (client, server) (rate(${SG_FAILED_TOTAL}{${CLIENT_FILTER}}[$__rate_interval]) or rate(${SG_FAILED_TOTAL}{${SERVER_FILTER}}[$__rate_interval]))`,
      )
      .legendFormat('{{client}} → {{server}}'),
  );

  panel.overrideByRegexp('/.*/', [
    { id: 'color', value: { fixedColor: 'red', mode: 'fixed' } },
  ]);

  return panel;
}
