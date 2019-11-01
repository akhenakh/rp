package rp

import (
	"container/list"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"
	"time"

	log "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/akhenakh/rp/cache"
)

// ProxyServer is a simple reverse lb proxy
type ProxyServer struct {
	// the transport used to query our backends
	transport http.RoundTripper

	// cache responses
	cache *cache.Cache

	mu sync.Mutex
	// list of backends addrs
	backends      *list.List
	pickedBackend *list.Element

	logger log.Logger
}

// Options to configure the proxy
type Options struct {
	// activate or disable the cache
	Cache bool
	// in memory lru cache size for cached responses
	CacheSize int
	// a logger
	Logger log.Logger
}

// New returns a ProxyServer that will load balance between backends
// and cache response in memory
func New(backends []string, opts *Options) (*ProxyServer, error) {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        100,
		IdleConnTimeout:     10 * time.Second,
		TLSHandshakeTimeout: 5 * time.Second,
		DisableCompression:  true,
	}

	ps := &ProxyServer{
		backends:  list.New(),
		transport: transport,
	}

	if opts != nil {
		ps.logger = opts.Logger

		if opts.Cache {
			cache, err := cache.New(opts.CacheSize, ps.logger)
			if err != nil {
				return nil, err
			}
			ps.cache = cache
		}
	}

	if ps.logger == nil {
		ps.logger = log.NewNopLogger()
	}
	ps.logger = log.With(ps.logger, "components", "ProxyServer")

	for _, b := range backends {
		ps.backends.PushBack(b)
	}

	return ps, nil
}

// PickBackend picks a round robin backend from the backends list
func (ps *ProxyServer) PickBackend() (string, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.pickedBackend == nil {
		ps.pickedBackend = ps.backends.Front()
	} else {
		pa := ps.pickedBackend
		ps.pickedBackend = pa.Next()
		if ps.pickedBackend == nil {
			ps.pickedBackend = ps.backends.Front()
		}
	}

	if ps.pickedBackend == nil {
		return "", fmt.Errorf("no backend available to pick")
	}

	return ps.pickedBackend.Value.(string), nil
}

func (ps *ProxyServer) handleError(rw http.ResponseWriter, err error) {
	errorCounter.Inc()
	http.Error(rw, err.Error(), http.StatusInternalServerError)
}

func (ps *ProxyServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	timer := prometheus.NewTimer(requestHistogram)
	defer timer.ObserveDuration()

	// lookup in the cache
	if ps.cache != nil {
		cresp, ok := ps.cache.GetResponse(req)
		if ok {
			for k, v := range cresp.Header {
				rw.Header().Add(k, v[0])
			}
			_, err := rw.Write(cresp.Body)
			if err != nil {
				ps.handleError(rw, err)
				level.Debug(ps.logger).Log("msg", "error while responding to client from cache", "req", req.URL)
			}
			return
		}
	}

	// pick a backend server
	backend, err := ps.PickBackend()
	if err != nil {
		ps.handleError(rw, err)
		return
	}

	// modify the request
	req.Host = backend
	req.URL.Scheme = "http"
	if req.URL.Host == "" {
		req.URL.Host = backend
	}

	level.Debug(ps.logger).Log("msg", "requesting", "req", req.URL, "backend", backend)

	resp, err := ps.transport.RoundTrip(req)
	if err != nil {
		ps.handleError(rw, err)
		level.Error(ps.logger).Log("msg", "error while querying backend",
			"req", req.URL, "backend", backend, "error", err)
		return
	}
	defer resp.Body.Close()

	copyHeader(rw.Header(), resp.Header)

	// simply pass the resp.Body if no cache
	if ps.cache == nil {
		_, err = io.Copy(rw, resp.Body)
		if err != nil {
			ps.handleError(rw, err)
			level.Error(ps.logger).Log("msg", "error while responding to client", "req", req.URL, "backend", backend)
			return
		}
		return
	}

	var bodyBytes []byte
	if resp.Body != nil {
		bodyBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			ps.handleError(rw, err)
			level.Error(ps.logger).Log("msg", "error while responding to client", "req", req.URL, "backend", backend)
			return
		}
	}

	// copy into the cache
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < 300 {
		ps.cache.PutResponse(resp, bodyBytes)
	}

	rw.Write(bodyBytes)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
