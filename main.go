package main

import (
	"context"
	"errors"
	"flag"
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
)

func init() {

}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	if *openToken == "" {
		return errors.New("-switchbot.open-token is required")
	}

	sc := switchbot.New(*openToken)

	http.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		log.Print("request")
		registry := prometheus.NewRegistry()

		target := r.FormValue("target")

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		devices, infrared, err := sc.Device().List(ctx)
		if err != nil {
			log.Printf("getting device list %v", err)
			return
		}

		deviceID2Names := prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "switchbot",
			Name:      "device",
		}, []string{"device_id", "device_name"})

		registry.MustRegister(deviceID2Names)

		for _, device := range devices {
			deviceID2Names.WithLabelValues(device.ID, device.Name).Set(0)
		}
		for _, device := range infrared {
			deviceID2Names.WithLabelValues(device.ID, device.Name).Set(0)
		}

		status, err := sc.Device().Status(ctx, target)
		if err != nil {
			log.Printf("getting device status: %v", err)
			return
		}

		switch status.Type {
		case switchbot.Meter:
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

			registry.MustRegister(meterHumidity)
			registry.MustRegister(meterTemperature)

			log.Printf("humidity: %d %%", status.Humidity)
			log.Printf("temperature: %f ℃", status.Temperature)
			meterHumidity.WithLabelValues(status.ID).Set(float64(status.Humidity))
			meterTemperature.WithLabelValues(status.ID).Set(status.Temperature)
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
