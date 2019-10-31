package rp

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	ErrorCounter = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "rp",
			Name:      "error_total",
			Help:      "The total number of errors occurring",
		},
	)
)
