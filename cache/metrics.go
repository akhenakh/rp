package cache

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HitCacheCounter hit cache count metric
	HitCacheCounter = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "rp",
			Name:      "cache_hit",
			Help:      "The total number of hit from cache",
		},
	)

	RequestCacheCounter = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "rp",
			Name:      "cache_req",
			Help:      "The total number of request through cache",
		},
	)
)
