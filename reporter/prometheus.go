package reporter

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PrometheusReporter struct {
	missCounter prometheus.Counter
	hitCounter  prometheus.Counter
}

func NewPrometheusReporter(
	serviceName string,
	cacheName string,
) *PrometheusReporter {
	return &PrometheusReporter{
		missCounter: NewPrometheusCounter(
			serviceName,
			cacheName,
			"miss",
		),
		hitCounter: NewPrometheusCounter(
			serviceName,
			cacheName,
			"hit",
		),
	}
}

func NewPrometheusCounter(
	serviceName string,
	cacheName string,
	counterType string,
) prometheus.Counter {
	return promauto.NewCounter(prometheus.CounterOpts{
		Name: fmt.Sprintf("%s_%s_total", serviceName, counterType),
		Help: "The total number of " + counterType,
		ConstLabels: map[string]string{
			"service": serviceName,
			"cache":   cacheName,
		},
	})
}
