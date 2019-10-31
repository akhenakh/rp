package rp

import (
	"container/list"
	"fmt"
	"io"
	"net/http"
	"sync"

	log "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	lru "github.com/hashicorp/golang-lru"
)

// ProxyServer is a simple reverse lb proxy
type ProxyServer struct {
	// the transport used to query our backends
	transport http.RoundTripper

	// cache responses
	cache *lru.ARCCache

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
	transport := http.DefaultTransport

	ps := &ProxyServer{
		backends:  list.New(),
		transport: transport,
	}

	if opts != nil {
		ps.logger = opts.Logger

		ps.logger = log.With(ps.logger, "components", "ProxyServer")
		if opts.Cache {
			cache, err := lru.NewARC(opts.CacheSize)
			if err != nil {
				return nil, err
			}
			ps.cache = cache
		}
	}

	if ps.logger == nil {
		ps.logger = log.NewNopLogger()
	}

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

func (ps *ProxyServer) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	//start := time.Now()
	if ps.cache != nil {
		// TODO: get from cache
		return
	}

	// defer func() {
	// 	t := time.Now()
	// 	elapsed := t.Sub(start)
	// }()

	backend, err := ps.PickBackend()
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	req.Host = backend
	req.URL.Scheme = "http"
	if req.URL.Host == "" {
		req.URL.Host = backend
	}

	level.Debug(ps.logger).Log("msg", "requesting", "req", req.URL, "backend", backend)

	resp, err := ps.transport.RoundTrip(req)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		level.Error(ps.logger).Log("msg", "error while querying backend",
			"req", req.URL, "backend", backend, "error", err)
		return
	}
	defer resp.Body.Close()

	copyHeader(rw.Header(), resp.Header)

	_, err = io.Copy(rw, resp.Body)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		// not critical
		level.Debug(ps.logger).Log("msg", "error while responding to client", "req", req.URL, "backend", backend)

		return
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
