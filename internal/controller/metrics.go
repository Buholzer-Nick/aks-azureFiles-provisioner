package controller

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"
)

// ReconcileMetrics captures controller reconcile metrics.
type ReconcileMetrics struct {
	total    *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// NewReconcileMetrics builds the metrics definitions.
func NewReconcileMetrics() *ReconcileMetrics {
	return &ReconcileMetrics{
		total: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "reconcile_total",
				Help: "Total number of reconcile attempts by result.",
			},
			[]string{"result"},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "reconcile_duration_seconds",
				Help:    "Reconcile duration in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"result"},
		),
	}
}

// Register registers the metrics with the provided registerer.
func (m *ReconcileMetrics) Register(registerer prometheus.Registerer) error {
	if m == nil {
		return nil
	}
	if registerer == nil {
		return errors.New("metrics registerer is nil")
	}

	if err := registerer.Register(m.total); err != nil {
		var already prometheus.AlreadyRegisteredError
		if !errors.As(err, &already) {
			return err
		}
	}
	if err := registerer.Register(m.duration); err != nil {
		var already prometheus.AlreadyRegisteredError
		if !errors.As(err, &already) {
			return err
		}
	}
	return nil
}

// Observe records a reconcile result and duration.
func (m *ReconcileMetrics) Observe(result string, seconds float64) {
	if m == nil {
		return
	}
	m.total.WithLabelValues(result).Inc()
	m.duration.WithLabelValues(result).Observe(seconds)
}
