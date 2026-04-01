import { PanelBuilder } from '@grafana/grafana-foundation-sdk/table';
import { PanelBuilder as BarGaugePanelBuilder } from '@grafana/grafana-foundation-sdk/bargauge';
import { DataqueryBuilder, PromQueryFormat } from '@grafana/grafana-foundation-sdk/prometheus';
import {
  TableFooterOptionsBuilder,
  TableSortByFieldStateBuilder,
  ReduceDataOptionsBuilder,
  BarGaugeDisplayMode,
  VizOrientation,
} from '@grafana/grafana-foundation-sdk/common';
import {
  ThresholdsConfigBuilder,
  ThresholdsMode,
  FieldColorBuilder,
  FieldColorModeId,
} from '@grafana/grafana-foundation-sdk/dashboard';

const DATASOURCE = { type: 'prometheus', uid: 'prometheus' } as const;

const SPAN_FILTER = 'service="documcp", span_kind=~"SPAN_KIND_SERVER|SPAN_KIND_INTERNAL"';

const TABLE_RATE_WINDOW = '$__range';

export function routesTablePanel(): PanelBuilder {
  return new PanelBuilder()
    .title('Requests by Route')
    .description('Top routes by request rate with error % and P95 latency')
    .gridPos({ h: 14, w: 14, x: 0, y: 10 })
    .datasource(DATASOURCE)
    .withTarget(
      new DataqueryBuilder()
        .refId('A')
        .expr(
          `topk(15, sum by (span_name) (rate(traces_spanmetrics_calls_total{${SPAN_FILTER}}[${TABLE_RATE_WINDOW}])))`,
        )
        .legendFormat('{{span_name}}')
        .instant()
        .format(PromQueryFormat.Table),
    )
    .withTarget(
      new DataqueryBuilder()
        .refId('B')
        .expr(
          `100 * sum by (span_name) (rate(traces_spanmetrics_calls_total{${SPAN_FILTER}, status_code="STATUS_CODE_ERROR"}[${TABLE_RATE_WINDOW}])) / sum by (span_name) (rate(traces_spanmetrics_calls_total{${SPAN_FILTER}}[${TABLE_RATE_WINDOW}]))`,
        )
        .legendFormat('{{span_name}}')
        .instant()
        .format(PromQueryFormat.Table),
    )
    .withTarget(
      new DataqueryBuilder()
        .refId('C')
        .expr(
          `histogram_quantile(0.95, sum by (span_name, le) (rate(traces_spanmetrics_latency_bucket{${SPAN_FILTER}}[${TABLE_RATE_WINDOW}])))`,
        )
        .legendFormat('{{span_name}}')
        .instant()
        .format(PromQueryFormat.Table),
    )
    .withTransformation({
      id: 'joinByField',
      options: { byField: 'span_name', mode: 'outer' },
    })
    .withTransformation({
      id: 'organize',
      options: {
        excludeByName: {
          Time: true,
          'Time 1': true,
          'Time 2': true,
          'Time 3': true,
        },
        renameByName: {
          span_name: 'Route',
          'Value #A': 'Rate',
          'Value #B': 'Error %',
          'Value #C': 'P95 Latency',
        },
      },
    })
    .showHeader(true)
    .sortBy([
      new TableSortByFieldStateBuilder()
        .displayName('Rate')
        .desc(true),
    ])
    .footer(new TableFooterOptionsBuilder())
    .overrideByName('Rate', [
      { id: 'unit', value: 'reqps' },
      { id: 'custom.cellOptions', value: { type: 'color-background', mode: 'gradient' } },
      { id: 'color', value: { mode: 'continuous-GrYlRd' } },
    ])
    .overrideByName('Error %', [
      { id: 'unit', value: 'percent' },
      {
        id: 'thresholds',
        value: {
          mode: 'absolute',
          steps: [
            { color: 'green', value: null },
            { color: 'red', value: 1 },
          ],
        },
      },
      { id: 'custom.cellOptions', value: { type: 'color-text' } },
      { id: 'color', value: { mode: 'thresholds' } },
    ])
    .overrideByName('P95 Latency', [
      { id: 'unit', value: 's' },
      {
        id: 'thresholds',
        value: {
          mode: 'absolute',
          steps: [
            { color: 'green', value: null },
            { color: 'yellow', value: 0.5 },
            { color: 'red', value: 2 },
          ],
        },
      },
      { id: 'custom.cellOptions', value: { type: 'color-text' } },
      { id: 'color', value: { mode: 'thresholds' } },
    ]);
}

export function slowestRoutesPanel(): BarGaugePanelBuilder {
  return new BarGaugePanelBuilder()
    .title('Slowest Routes (P95)')
    .description('Top 10 routes by P95 latency')
    .gridPos({ h: 14, w: 10, x: 14, y: 10 })
    .datasource(DATASOURCE)
    .withTarget(
      new DataqueryBuilder()
        .refId('A')
        .expr(
          `topk(10, histogram_quantile(0.95, sum by (span_name, le) (rate(traces_spanmetrics_latency_bucket{${SPAN_FILTER}}[${TABLE_RATE_WINDOW}]))))`,
        )
        .legendFormat('{{span_name}}'),
    )
    .unit('s')
    .thresholds(
      new ThresholdsConfigBuilder()
        .mode(ThresholdsMode.Absolute)
        .steps([
          { color: 'green', value: null as unknown as number },
          { color: 'yellow', value: 0.5 },
          { color: 'red', value: 2 },
        ]),
    )
    .reduceOptions(
      new ReduceDataOptionsBuilder().calcs(['lastNotNull']),
    )
    .orientation(VizOrientation.Horizontal)
    .displayMode(BarGaugeDisplayMode.Gradient)
    .showUnfilled(true)
    .minVizWidth(8)
    .minVizHeight(10)
    .colorScheme(
      new FieldColorBuilder().mode(FieldColorModeId.ContinuousGrYlRd),
    );
}
