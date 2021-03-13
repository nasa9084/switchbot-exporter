# switchbot-exporter

## Prometheus Configuration

The switchbot exporter needs to be passed the target ID as a parameter, this can be done with relabelling (like blackbox exporter)

Example Config:

``` yaml
scrape_configs:
  - job_name: 'switchbox'
    metrics_path: /metrics
    static_configs:
      - targets:
        - DFA0029F2622 # Target switchbot meter
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: 127.0.0.1:8080 # The switchbot exporter's real ip/port
```
