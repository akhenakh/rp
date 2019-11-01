package cache

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// hitCacheCounter hit cache count metric
	hitCacheCounter = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "rp",
			Name:      "cache_hit",
			Help:      "The total number of hit from cache",
		},
	)

	requestCacheCounter = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "rp",
			Name:      "cache_req",
			Help:      "The total number of request through cache",
		},
	)
)
