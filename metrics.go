package rp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	errorCounter = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "rp",
			Name:      "error_total",
			Help:      "The total number of errors occurring",
		},
	)

	requestHistogram = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "rp",
			Name:      "request_duration",
			Help:      "Request duration",
		},
	)
)
