package infra

const httpClientDashboardJSON = `{
  "annotations": {
    "list": []
  },
  "editable": true,
  "fiscalYearStartMonth": 0,
  "graphTooltip": 1,
  "id": null,
  "links": [],
  "liveNow": false,
  "panels": [
    {
      "collapsed": false,
      "gridPos": { "h": 1, "w": 24, "x": 0, "y": 0 },
      "id": 100,
      "title": "Overview & KPIs",
      "type": "row"
    },
    {
      "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
      "fieldConfig": {
        "defaults": {
          "color": { "mode": "palette-classic" },
          "mappings": [],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              { "color": "blue", "value": null }
            ]
          }
        },
        "overrides": []
      },
      "gridPos": { "h": 4, "w": 6, "x": 0, "y": 1 },
      "id": 1,
      "options": {
        "colorMode": "value",
        "graphMode": "area",
        "justifyMode": "auto",
        "orientation": "auto",
        "reduceOptions": {
          "calcs": ["lastNotNull"],
          "fields": "",
          "values": false
        }
      },
      "pluginVersion": "10.0.0",
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
          "editorMode": "code",
          "expr": "sum(http_client_request_duration_seconds_count{exported_job=~\"$service\", http_route=~\"$route\", otel_scope_name=~\".*httpclient\"}) or vector(0)",
          "instant": false,
          "legendFormat": "Total Requests",
          "range": true,
          "refId": "A"
        }
      ],
      "title": "Total Requests",
      "type": "stat"
    },
    {
      "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
      "fieldConfig": {
        "defaults": {
          "color": { "mode": "palette-classic" },
          "mappings": [],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              { "color": "green", "value": null },
              { "color": "yellow", "value": 0.5 },
              { "color": "red", "value": 2.0 }
            ]
          },
          "unit": "s"
        },
        "overrides": []
      },
      "gridPos": { "h": 4, "w": 6, "x": 6, "y": 1 },
      "id": 2,
      "options": {
        "colorMode": "value",
        "graphMode": "area",
        "justifyMode": "auto",
        "orientation": "auto",
        "reduceOptions": {
          "calcs": ["mean"],
          "fields": "",
          "values": false
        }
      },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
          "editorMode": "code",
          "expr": "(sum(rate(http_client_request_duration_seconds_sum{exported_job=~\"$service\", http_route=~\"$route\"}[1m])) / sum(rate(http_client_request_duration_seconds_count{exported_job=~\"$service\", http_route=~\"$route\"}[1m]))) or vector(0)",
          "instant": false,
          "legendFormat": "Avg Latency",
          "range": true,
          "refId": "A"
        }
      ],
      "title": "Average Latency",
      "type": "stat"
    },
    {
      "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
      "fieldConfig": {
        "defaults": {
          "color": { "mode": "palette-classic" },
          "mappings": [],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              { "color": "green", "value": null },
              { "color": "yellow", "value": 1.0 },
              { "color": "red", "value": 3.0 }
            ]
          },
          "unit": "s"
        },
        "overrides": []
      },
      "gridPos": { "h": 4, "w": 6, "x": 12, "y": 1 },
      "id": 3,
      "options": {
        "colorMode": "value",
        "graphMode": "area",
        "justifyMode": "auto",
        "orientation": "auto",
        "reduceOptions": {
          "calcs": ["mean"],
          "fields": "",
          "values": false
        }
      },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
          "editorMode": "code",
          "expr": "histogram_quantile(0.95, sum by (le) (rate(http_client_request_duration_seconds_bucket{exported_job=~\"$service\", http_route=~\"$route\"}[1m]))) or vector(0)",
          "instant": false,
          "legendFormat": "P95 Latency",
          "range": true,
          "refId": "A"
        }
      ],
      "title": "P95 Latency",
      "type": "stat"
    },
    {
      "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
      "fieldConfig": {
        "defaults": {
          "color": { "mode": "palette-classic" },
          "mappings": [],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              { "color": "green", "value": null },
              { "color": "yellow", "value": 1.0 },
              { "color": "red", "value": 5.0 }
            ]
          },
          "unit": "percent"
        },
        "overrides": []
      },
      "gridPos": { "h": 4, "w": 6, "x": 18, "y": 1 },
      "id": 4,
      "options": {
        "colorMode": "value",
        "graphMode": "area",
        "justifyMode": "auto",
        "orientation": "auto",
        "reduceOptions": {
          "calcs": ["mean"],
          "fields": "",
          "values": false
        }
      },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
          "editorMode": "code",
          "expr": "((sum(rate(http_client_request_duration_seconds_count{exported_job=~\"$service\", http_route=~\"$route\", http_response_status_code=~\"[45]..\"}[1m])) / sum(rate(http_client_request_duration_seconds_count{exported_job=~\"$service\", http_route=~\"$route\"}[1m]))) * 100) or vector(0)",
          "instant": false,
          "legendFormat": "Error %",
          "range": true,
          "refId": "A"
        }
      ],
      "title": "Error Rate %",
      "type": "stat"
    },
    {
      "collapsed": false,
      "gridPos": { "h": 1, "w": 24, "x": 0, "y": 5 },
      "id": 101,
      "title": "Traffic & Latency Over Time",
      "type": "row"
    },
    {
      "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
      "fieldConfig": {
        "defaults": {
          "color": { "mode": "palette-classic" },
          "custom": {
            "axisBorderShow": false,
            "axisCenteredZero": false,
            "axisColorMode": "text",
            "axisLabel": "req/s",
            "axisPlacement": "auto",
            "fillOpacity": 15,
            "gradientMode": "opacity",
            "lineWidth": 2,
            "spanNulls": false
          },
          "unit": "reqps"
        },
        "overrides": []
      },
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 6 },
      "id": 5,
      "options": {
        "legend": { "calcs": ["mean", "max"], "displayMode": "table", "placement": "bottom" },
        "tooltip": { "mode": "multi", "sort": "desc" }
      },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
          "editorMode": "code",
          "expr": "sum by (http_request_method, http_route) (rate(http_client_request_duration_seconds_count{exported_job=~\"$service\", http_route=~\"$route\"}[1m]))",
          "legendFormat": "{{http_request_method}} {{http_route}}",
          "refId": "A"
        }
      ],
      "title": "Request Rate (Throughput)",
      "type": "timeseries"
    },
    {
      "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
      "fieldConfig": {
        "defaults": {
          "color": { "mode": "palette-classic" },
          "custom": {
            "axisBorderShow": false,
            "axisCenteredZero": false,
            "axisColorMode": "text",
            "axisLabel": "latency",
            "axisPlacement": "auto",
            "fillOpacity": 15,
            "gradientMode": "opacity",
            "lineWidth": 2,
            "spanNulls": false
          },
          "unit": "s"
        },
        "overrides": []
      },
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 6 },
      "id": 6,
      "options": {
        "legend": { "calcs": ["mean", "max"], "displayMode": "table", "placement": "bottom" },
        "tooltip": { "mode": "multi", "sort": "desc" }
      },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
          "editorMode": "code",
          "expr": "sum by (http_route) (rate(http_client_request_duration_seconds_sum{exported_job=~\"$service\", http_route=~\"$route\"}[1m])) / sum by (http_route) (rate(http_client_request_duration_seconds_count{exported_job=~\"$service\", http_route=~\"$route\"}[1m]))",
          "legendFormat": "{{http_route}}",
          "refId": "A"
        }
      ],
      "title": "Average Latency by Route",
      "type": "timeseries"
    },
    {
      "collapsed": false,
      "gridPos": { "h": 1, "w": 24, "x": 0, "y": 14 },
      "id": 102,
      "title": "Errors & Response Status Breakdown",
      "type": "row"
    },
    {
      "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
      "fieldConfig": {
        "defaults": {
          "color": { "mode": "palette-classic" },
          "custom": {
            "axisBorderShow": false,
            "axisCenteredZero": false,
            "axisColorMode": "text",
            "axisLabel": "errors/s",
            "axisPlacement": "auto",
            "fillOpacity": 20,
            "gradientMode": "opacity",
            "lineWidth": 2,
            "spanNulls": false
          },
          "unit": "reqps"
        },
        "overrides": []
      },
      "gridPos": { "h": 8, "w": 14, "x": 0, "y": 15 },
      "id": 7,
      "options": {
        "legend": { "calcs": ["sum"], "displayMode": "table", "placement": "bottom" },
        "tooltip": { "mode": "multi", "sort": "desc" }
      },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
          "editorMode": "code",
          "expr": "sum by (http_route, http_response_status_code) (rate(http_client_request_duration_seconds_count{exported_job=~\"$service\", http_route=~\"$route\", http_response_status_code=~\"[45]..\"}[1m]))",
          "legendFormat": "{{http_response_status_code}} {{http_route}}",
          "refId": "A"
        }
      ],
      "title": "Errors (4xx / 5xx) by Route",
      "type": "timeseries"
    },
    {
      "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
      "fieldConfig": {
        "defaults": {
          "color": { "mode": "palette-classic" },
          "mappings": []
        },
        "overrides": []
      },
      "gridPos": { "h": 8, "w": 10, "x": 14, "y": 15 },
      "id": 8,
      "options": {
        "legend": { "displayMode": "table", "placement": "right", "values": ["value", "percent"] },
        "pieType": "donut",
        "reduceOptions": {
          "calcs": ["lastNotNull"],
          "fields": "",
          "values": false
        },
        "tooltip": { "mode": "single", "sort": "none" }
      },
      "targets": [
        {
          "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
          "editorMode": "code",
          "expr": "sum by (http_response_status_code) (http_client_request_duration_seconds_count{exported_job=~\"$service\", http_route=~\"$route\"})",
          "legendFormat": "HTTP {{http_response_status_code}}",
          "refId": "A"
        }
      ],
      "title": "Status Code Distribution",
      "type": "piechart"
    }
  ],
  "refresh": "5s",
  "schemaVersion": 38,
  "style": "dark",
  "tags": ["godev", "httpclient", "rest", "telemetry"],
  "templating": {
    "list": [
      {
        "current": { "selected": true, "text": "All", "value": ".*" },
        "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
        "definition": "label_values(http_client_request_duration_seconds_count, exported_job)",
        "hide": 0,
        "includeAll": true,
        "multi": false,
        "name": "service",
        "options": [],
        "query": { "query": "label_values(http_client_request_duration_seconds_count, exported_job)", "refId": "StandardVariableQuery" },
        "refresh": 1,
        "regex": "",
        "skipUrlSync": false,
        "sort": 1,
        "type": "query"
      },
      {
        "current": { "selected": true, "text": "All", "value": ".*" },
        "datasource": { "type": "prometheus", "uid": "PBFA97CFB590B2093" },
        "definition": "label_values(http_client_request_duration_seconds_count{exported_job=~\"$service\"}, http_route)",
        "hide": 0,
        "includeAll": true,
        "multi": true,
        "name": "route",
        "options": [],
        "query": { "query": "label_values(http_client_request_duration_seconds_count{exported_job=~\"$service\"}, http_route)", "refId": "StandardVariableQuery" },
        "refresh": 1,
        "regex": "",
        "skipUrlSync": false,
        "sort": 1,
        "type": "query"
      }
    ]
  },
  "time": {
    "from": "now-15m",
    "to": "now"
  },
  "timepicker": {
    "refresh_intervals": ["5s", "10s", "30s", "1m", "5m"]
  },
  "timezone": "browser",
  "title": "HTTP Client Telemetry",
  "uid": "http-client-telemetry",
  "version": 1
}`
