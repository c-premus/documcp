import { PanelBuilder } from '@grafana/grafana-foundation-sdk/table';
import { PanelBuilder as NodeGraphPanelBuilder } from '@grafana/grafana-foundation-sdk/nodegraph';
import { TempoQueryBuilder, TempoQueryType, SearchTableType } from '@grafana/grafana-foundation-sdk/tempo';
import {
  TableFooterOptionsBuilder,
  TableSortByFieldStateBuilder,
} from '@grafana/grafana-foundation-sdk/common';
import type { DynamicConfigValue } from '@grafana/grafana-foundation-sdk/dashboard';

const TEMPO_DATASOURCE = { type: 'tempo' as const, uid: 'tempo' };

const TRACE_QUERY =
  '{ resource.service.name = "DocuMCP" && kind = server }';

const TRACE_EXPLORE_URL = [
  '/explore?orgId=1&left=%7B%22datasource%22:%22tempo%22,',
  '%22queries%22:%5B%7B%22refId%22:%22A%22,',
  '%22datasource%22:%7B%22type%22:%22tempo%22,',
  '%22uid%22:%22tempo%22%7D,',
  '%22queryType%22:%22traceql%22,',
  '%22query%22:%22${__value.raw}%22%7D%5D%7D',
].join('');

export function recentTracesPanel(): PanelBuilder {
  const durationOverrides: DynamicConfigValue[] = [
    { id: 'unit', value: 'ns' },
    {
      id: 'thresholds',
      value: {
        mode: 'absolute',
        steps: [
          { color: 'green', value: null },
          { color: 'yellow', value: 500_000_000 },
          { color: 'red', value: 2_000_000_000 },
        ],
      },
    },
    { id: 'custom.cellOptions', value: { type: 'color-text' } },
    { id: 'color', value: { mode: 'thresholds' } },
  ];

  const traceIdOverrides: DynamicConfigValue[] = [
    {
      id: 'links',
      value: [
        {
          title: 'View Trace',
          url: TRACE_EXPLORE_URL,
        },
      ],
    },
  ];

  return new PanelBuilder()
    .title('Recent Traces')
    .description(
      'Recent traces from Tempo — click trace ID to explore',
    )
    .gridPos({ h: 10, w: 24, x: 0, y: 52 })
    .datasource(TEMPO_DATASOURCE)
    .withTarget(
      new TempoQueryBuilder()
        .refId('A')
        .datasource(TEMPO_DATASOURCE)
        .queryType('traceql')
        .query(TRACE_QUERY)
        .limit(100)
        .tableType(SearchTableType.Spans),
    )
    .showHeader(true)
    .sortBy([
      new TableSortByFieldStateBuilder()
        .displayName('Start Time')
        .desc(true),
    ])
    .footer(
      new TableFooterOptionsBuilder()
        .show(true)
        .enablePagination(true)
        .countRows(true),
    )
    .overrideByName('Duration', durationOverrides)
    .overrideByName('Trace ID', traceIdOverrides);
}

export function serviceMapPanel(): NodeGraphPanelBuilder {
  return new NodeGraphPanelBuilder()
    .title('Service Map')
    .description(
      'Service dependency graph for DocuMCP stack derived from Tempo service graph metrics',
    )
    .gridPos({ h: 12, w: 24, x: 0, y: 62 })
    .datasource(TEMPO_DATASOURCE)
    .withTarget(
      new TempoQueryBuilder()
        .refId('A')
        .datasource(TEMPO_DATASOURCE)
        .queryType(TempoQueryType.ServiceMap),
    );
}
