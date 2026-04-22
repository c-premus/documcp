import { PanelBuilder as TimeseriesPanelBuilder } from '@grafana/grafana-foundation-sdk/timeseries';
import { PanelBuilder as LogsPanelBuilder } from '@grafana/grafana-foundation-sdk/logs';
import { DataqueryBuilder } from '@grafana/grafana-foundation-sdk/loki';
import {
  GraphDrawStyle,
  LegendDisplayMode,
  LegendPlacement,
  LogsDedupStrategy,
  LogsSortOrder,
  SortOrder,
  StackingMode,
  TooltipDisplayMode,
  VisibilityMode,
  VizLegendOptionsBuilder,
  VizTooltipOptionsBuilder,
  StackingConfigBuilder,
} from '@grafana/grafana-foundation-sdk/common';

const LOKI_DATASOURCE = { type: 'loki', uid: 'loki' } as const;

const LEVEL_COLORS: ReadonlyArray<{ name: string; color: string }> = [
  { name: 'ERROR', color: 'red' },
  { name: 'WARNING', color: 'orange' },
  { name: 'INFO', color: 'green' },
  { name: 'DEBUG', color: 'blue' },
];

export function logVolumePanel(): TimeseriesPanelBuilder {
  const query = new DataqueryBuilder()
    .refId('A')
    .expr('sum by (level) (count_over_time({service_name="documcp-app"} [$__auto]))')
    .legendFormat('{{level}}');

  const legend = new VizLegendOptionsBuilder()
    .displayMode(LegendDisplayMode.List)
    .placement(LegendPlacement.Bottom);

  const tooltip = new VizTooltipOptionsBuilder()
    .mode(TooltipDisplayMode.Multi)
    .sort(SortOrder.Descending);

  const stacking = new StackingConfigBuilder()
    .mode(StackingMode.Normal)
    .group('A');

  const panel = new TimeseriesPanelBuilder()
    .title('Log Volume by Level')
    .description('Log lines per second grouped by severity')
    .gridPos({ h: 6, w: 24, x: 0, y: 0 })
    .datasource(LOKI_DATASOURCE)
    .withTarget(query)
    .unit('short')
    .drawStyle(GraphDrawStyle.Bars)
    .fillOpacity(80)
    .stacking(stacking)
    .showPoints(VisibilityMode.Never)
    .legend(legend)
    .tooltip(tooltip);

  for (const level of LEVEL_COLORS) {
    panel.overrideByName(level.name, [
      { id: 'color', value: { fixedColor: level.color, mode: 'fixed' } },
    ]);
  }

  return panel;
}

export function recentLogsPanel(): LogsPanelBuilder {
  const query = new DataqueryBuilder()
    .refId('A')
    .expr('{service_name="documcp-app"}')
    .maxLines(200);

  return new LogsPanelBuilder()
    .title('Recent Logs')
    .description('DocuMCP application logs with trace correlation links')
    .gridPos({ h: 20, w: 24, x: 0, y: 0 })
    .datasource(LOKI_DATASOURCE)
    .withTarget(query)
    .showTime(true)
    .showLabels(true)
    .showCommonLabels(false)
    .wrapLogMessage(true)
    .prettifyLogMessage(true)
    .enableLogDetails(true)
    .sortOrder(LogsSortOrder.Descending)
    .dedupStrategy(LogsDedupStrategy.None);
}
