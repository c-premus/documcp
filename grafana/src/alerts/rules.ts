// Grafana unified-alerting provisioning format, generated as JSON. Grafana
// 10+ accepts JSON or YAML at provisioning/alerting/*. JSON keeps the
// pattern aligned with the dashboard generator (dist/documcp.json) and
// survives `git diff --exit-code` diffing without YAML-library drift.
//
// Datasource UID: the placeholder 'prometheus' matches the default
// Prometheus datasource shipped in the deploy compose. Operators with a
// different UID must edit the provisioning file before loading, or set a
// Grafana env var to remap.

interface PrometheusQuery {
  refId: string;
  datasourceUid: string;
  relativeTimeRange?: { from: number; to: number };
  model: {
    refId: string;
    expr: string;
    instant: boolean;
    intervalMs?: number;
    maxDataPoints?: number;
  };
}

interface ThresholdQuery {
  refId: string;
  datasourceUid: '__expr__';
  model: {
    refId: string;
    type: 'threshold';
    expression: string;
    conditions: Array<{
      type: 'query';
      evaluator: { type: 'eq' | 'gt' | 'lt'; params: number[] };
      operator: { type: 'and' };
      query: { params: string[] };
      reducer: { type: 'last' };
    }>;
    intervalMs?: number;
    maxDataPoints?: number;
  };
}

interface AlertRule {
  uid: string;
  title: string;
  condition: string;
  data: Array<PrometheusQuery | ThresholdQuery>;
  noDataState: 'Alerting' | 'NoData' | 'OK';
  execErrState: 'Alerting' | 'Error' | 'OK';
  for: string;
  annotations: Record<string, string>;
  labels: Record<string, string>;
  isPaused: boolean;
}

interface RuleGroup {
  orgId: number;
  name: string;
  folder: string;
  interval: string;
  rules: AlertRule[];
}

interface AlertsFile {
  apiVersion: 1;
  groups: RuleGroup[];
}

// Default Prometheus datasource UID for Grafana provisioning. Must match
// the datasource declared in the deploy compose. If the operator's
// environment uses a different UID, edit provisioning/alerting/*.json
// after copying or override via Grafana GF_DEFAULT_DATASOURCE_UID.
const PROM_DATASOURCE_UID = 'prometheus';

// prometheusQuery returns a query part that reads the named metric on
// an instant query. 5-minute relative range gives Grafana enough data
// for the "last" reducer even during brief scrape gaps.
function prometheusQuery(refId: string, expr: string): PrometheusQuery {
  return {
    refId,
    datasourceUid: PROM_DATASOURCE_UID,
    relativeTimeRange: { from: 300, to: 0 },
    model: {
      refId,
      expr,
      instant: true,
    },
  };
}

// eqThreshold returns an expression part that fires when the named
// query's last value equals target. Pair with a Prometheus query that
// emits a 0/1 gauge.
function eqThreshold(refId: string, query: string, target: number): ThresholdQuery {
  return {
    refId,
    datasourceUid: '__expr__',
    model: {
      refId,
      type: 'threshold',
      expression: query,
      conditions: [
        {
          type: 'query',
          evaluator: { type: 'eq', params: [target] },
          operator: { type: 'and' },
          query: { params: [query] },
          reducer: { type: 'last' },
        },
      ],
    },
  };
}

const noRiverLeader: AlertRule = {
  uid: 'documcp_no_river_leader',
  title: 'DocuMCP — no River leader',
  condition: 'C',
  data: [
    prometheusQuery('A', 'documcp_river_leader_active'),
    eqThreshold('C', 'A', 0),
  ],
  // If river_leader is unscrapeable we cannot prove periodic jobs are
  // firing; treat as Alerting so an outage does not silently pass the gate.
  noDataState: 'Alerting',
  execErrState: 'Alerting',
  for: '5m',
  annotations: {
    summary: 'No River leader for 5 minutes',
    description:
      'documcp_river_leader_active = 0 — no DocuMCP replica holds the River leadership lease. ' +
      'Periodic jobs (oauth token cleanup, orphan file cleanup, soft-delete purge, scope grant cleanup) ' +
      'are not firing. Check that at least one replica runs with --with-worker or as a worker pod, ' +
      'and that the postgres river_leader table is reachable.',
  },
  labels: {
    severity: 'critical',
    service: 'documcp',
  },
  isPaused: false,
};

const readinessFailing: AlertRule = {
  uid: 'documcp_readiness_failing',
  title: 'DocuMCP — readiness failing',
  condition: 'C',
  data: [
    prometheusQuery('A', 'documcp_ready'),
    eqThreshold('C', 'A', 0),
  ],
  // NoData means no DocuMCP instance is exposing /metrics — alert so the
  // outage is visible even when the application is entirely unreachable.
  noDataState: 'Alerting',
  execErrState: 'Alerting',
  for: '2m',
  annotations: {
    summary: 'DocuMCP readiness failing for 2 minutes',
    description:
      'documcp_ready = 0 — the self-collecting readiness gauge reports ' +
      'that Postgres or Redis is not responding to Ping on the uninstrumented pool. ' +
      'Check /health/ready JSON response for the specific dependency status; ' +
      'inspect documcp_db_open_connections and documcp_redis_pool_misses_total for corroboration.',
  },
  labels: {
    severity: 'critical',
    service: 'documcp',
  },
  isPaused: false,
};

// buildDocuMCPAlerts returns the full alert provisioning file.
export function buildDocuMCPAlerts(): AlertsFile {
  return {
    apiVersion: 1,
    groups: [
      {
        orgId: 1,
        name: 'documcp',
        folder: 'DocuMCP',
        // Evaluate every 30s — matches the Prometheus scrape cadence of
        // the underlying gauges. Shorter would burn CPU without catching
        // a faster failure signal.
        interval: '30s',
        rules: [noRiverLeader, readinessFailing],
      },
    ],
  };
}
