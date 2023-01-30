package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	switchbot "github.com/nasa9084/go-switchbot"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	listenAddress = flag.String("web.listen-address", ":8080", "The address to listen on for HTTP requests")
	openToken     = flag.String("switchbot.open-token", "", "The open token for switchbot-api")
	secretKey     = flag.String("switchbot.secret-key", "", "The secret key for switchbot-api")
)

// deviceLabels is global cache gauge which stores device id and device name as its label.
var deviceLabels = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Namespace: "switchbot",
	Name:      "device",
}, []string{"device_id", "device_name"})

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	openTokenFromEnv := os.Getenv("SWITCHBOT_OPENTOKEN")
	if openTokenFromEnv != "" {
		*openToken = openTokenFromEnv
	}

	if *openToken == "" {
		return errors.New("-switchbot.open-token is required")
	}

	secretKeyFromEnv := os.Getenv("SWITCHBOT_SECRETKEY")
	if secretKeyFromEnv != "" {
		*secretKey = secretKeyFromEnv
	}

	if *secretKey == "" {
		return errors.New("-switchbot.secret-key is required")
	}

	sc := switchbot.New(*openToken, *secretKey)

	if err := reloadDevices(sc); err != nil {
		return err
	}

	hup := make(chan os.Signal, 1)
	reloadCh := make(chan chan error)
	signal.Notify(hup, syscall.SIGHUP)

	go func() {
		// reload
		for {
			select {
			case <-hup:
				if err := reloadDevices(sc); err != nil {
					log.Printf("error reloading devices: %v", err)
				}
				log.Print("reloaded devices")
			case errCh := <-reloadCh:
				if err := reloadDevices(sc); err != nil {
					log.Printf("error relaoding devices: %v", err)
					errCh <- err
				} else {
					errCh <- nil
				}
				log.Print("relaoded devices")
			}
		}
	}()

	http.HandleFunc("/-/reload", func(w http.ResponseWriter, r *http.Request) {
		if expectMethod := http.MethodPost; r.Method != expectMethod {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintf(w, "This endpoint requires a %s request.\n", expectMethod)
			return
		}

		rc := make(chan error)
		reloadCh <- rc
		if err := <-rc; err != nil {
			http.Error(w, fmt.Sprintf("failed to reload config: %s", err), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		registry := prometheus.NewRegistry()
		target := r.FormValue("target")

		if target == "" {
			http.Error(w, "target parameter is missing", http.StatusBadRequest)
			return
		}

		status, err := sc.Device().Status(r.Context(), target)
		if err != nil {
			log.Printf("getting device status: %v", err)
			return
		}

		switch status.Type {
		case switchbot.Meter, switchbot.MeterPlus:
			meterHumidity := prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: "switchbot",
				Subsystem: "meter",
				Name:      "humidity",
			}, []string{"device_id"})

			meterTemperature := prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: "switchbot",
				Subsystem: "meter",
				Name:      "temperature",
			}, []string{"device_id"})

			registry.MustRegister(deviceLabels) // register global device labels cache
			registry.MustRegister(meterHumidity, meterTemperature)

			meterHumidity.WithLabelValues(status.ID).Set(float64(status.Humidity))
			meterTemperature.WithLabelValues(status.ID).Set(status.Temperature)
		case switchbot.PlugMiniJP:
			plugWeight := prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: "switchbot",
				Subsystem: "plug",
				Name:      "weight",
			}, []string{"device_id"})

			plugVoltage := prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Namespace: "switchbot",
				Subsystem: "plug",
				Name:      "voltage",
			}, []string{"device_id"})

			registry.MustRegister(deviceLabels)
			registry.MustRegister(plugWeight, plugVoltage)
			plugWeight.WithLabelValues(status.ID).Set(status.Weight)
			plugVoltage.WithLabelValues(status.ID).Set(status.Voltage)
		}

		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	})

	srv := &http.Server{Addr: *listenAddress}
	srvc := make(chan error)
	term := make(chan os.Signal, 1)
	signal.Notify(term, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			srvc <- err
		}
	}()

	for {
		select {
		case <-term:
			log.Print("received terminate signal")
			return nil
		case err := <-srvc:
			return err
		}
	}
}

func reloadDevices(sc *switchbot.Client) error {
	log.Print("reload device list")
	devices, infrared, err := sc.Device().List(context.Background())
	if err != nil {
		return fmt.Errorf("getting device list: %w", err)
	}

	for _, device := range devices {
		deviceLabels.WithLabelValues(device.ID, device.Name).Set(0)
	}
	for _, device := range infrared {
		deviceLabels.WithLabelValues(device.ID, device.Name).Set(0)
	}

	return nil
}
