# switchbot-exporter

Exports [switchbot](https://us.switch-bot.com) device metrics for [prometheus](https://prometheus.io).

## Supported Devices / Metrics

Currently supports humidity and temperature for:
* Hub 2
* Humidifier
* Meter
* Meter Plus
* Indoor/Outdoor Thermo-Hygrometer

Supports weight and voltage for:
* Plug Mini (JP)

## Prometheus Configuration

### Static Configuration

The switchbot exporter needs to be passed the target ID as a parameter, this can be done with relabelling (like [blackbox exporter](https://github.com/prometheus/blackbox_exporter)).

Change the host:port in the relabel_configs `replacement` to the host:port where the exporter is listening.

#### Example Config (Static Configs):

``` yaml
scrape_configs:
  - job_name: 'switchbot'
    scrape_interval: 5m # not to reach API rate limit
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
### Dynamic Configuration using Service Discovery

The switchbot exporter also implements http service discovery to create a prometheus target for each supported device in your account.  When using service discover, the `static_configs` is not needed.  Relabeling is used (see [blackbox exporter](https://github.com/prometheus/blackbox_exporter)) to convert the device's id into a url with the id as the url's target query parameter.

Change the host:port in the http_sd_configs `url` and in the relabel_configs `replacement` to the host:port where the exporter is listening.

#### Example Config (Dynamic Configs):

``` yaml
scrape_configs:
  - job_name: 'switchbot'
    scrape_interval: 5m # not to reach API rate limit
    metrics_path: /metrics
    http_sd_configs:
    - url: http://127.0.0.1:8080/discover
      refresh_interval: 1d # no need to check for new devices very often
    relabel_configs:
      - source_labels: [__address__]
        target_label: __param_target
      - source_labels: [__param_target]
        target_label: instance
      - target_label: __address__
        replacement: 127.0.0.1:8080 # The switchbot exporter's real ip/port
```

## Limitation

Only a subset of switchbot devices are currently supported.

[switchbot API's request limit](https://github.com/OpenWonderLabs/SwitchBotAPI#request-limit)
