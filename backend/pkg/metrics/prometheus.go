package metrics

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds all Prometheus metrics for the kvstore.
type Metrics struct {
	Registry *prometheus.Registry

	PutsTotal    prometheus.Counter
	GetsTotal    prometheus.Counter
	DeletesTotal prometheus.Counter
	ErrorsTotal  *prometheus.CounterVec

	OperationDuration *prometheus.HistogramVec

	MemtableSizeBytes prometheus.Gauge
	WALSizeBytes      prometheus.Gauge
}

// NewMetrics creates and registers all metrics on a fresh registry.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	m := &Metrics{
		Registry: reg,

		PutsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "kvstore_puts_total",
			Help: "Total number of Put operations",
		}),
		GetsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "kvstore_gets_total",
			Help: "Total number of Get operations",
		}),
		DeletesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "kvstore_deletes_total",
			Help: "Total number of Delete operations",
		}),
		ErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "kvstore_errors_total",
			Help: "Total number of errors by operation type",
		}, []string{"op"}),

		OperationDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "kvstore_operation_duration_seconds",
			Help:    "Duration of storage operations in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"op"}),

		MemtableSizeBytes: prometheus.Gauge(prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "kvstore_memtable_size_bytes",
			Help: "Approximate size of the memtable in bytes",
		})),
		WALSizeBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "kvstore_wal_size_bytes",
			Help: "Current size of the WAL file in bytes",
		}),
	}

	reg.MustRegister(
		m.PutsTotal,
		m.GetsTotal,
		m.DeletesTotal,
		m.ErrorsTotal,
		m.OperationDuration,
		m.MemtableSizeBytes,
		m.WALSizeBytes,
	)

	return m
}
