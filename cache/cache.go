package cache

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"

	log "github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	lru "github.com/hashicorp/golang-lru"
)

const CachedHeader = "X-Cached-response"

// Cache is a quick and dirty LRU cache not handling proper HTTP caching
type Cache struct {
	*lru.ARCCache
	logger log.Logger
}

// CachedResponse a cached http response
type CachedResponse struct {
	Header http.Header
	Body   io.ReadCloser
}

// New returns a new Cache
func New(size int, logger log.Logger) (*Cache, error) {
	cache, err := lru.NewARC(size)
	if err != nil {
		return nil, err
	}
	c := &Cache{ARCCache: cache}

	c.logger = logger
	if c.logger == nil {
		c.logger = log.NewNopLogger()
		c.logger = log.With(c.logger, "components", "Cache")
	}

	return c, nil
}

// GetResponse get a response from the cache using req.URL as key
func (c *Cache) GetResponse(req *http.Request) (*CachedResponse, bool) {
	level.Debug(c.logger).Log("msg", "requesting from the cache", "key", req.RequestURI)

	RequestCacheCounter.Inc()
	v, ok := c.Get(req.RequestURI)
	if ok {
		HitCacheCounter.Inc()
		cresp := v.(*CachedResponse)
		return cresp, true
	}
	return nil, false
}

// PutResponse adds a response to the cache
func (c *Cache) PutResponse(resp *http.Response, body []byte) {
	cr := &CachedResponse{
		Body:   ioutil.NopCloser(bytes.NewBuffer(body)),
		Header: make(http.Header),
	}
	copyHeader(cr.Header, resp.Header)
	cr.Header.Add(CachedHeader, "true")
	c.Add(resp.Request.RequestURI, cr)
	level.Debug(c.logger).Log("msg", "putting a response into the cache", "key", resp.Request.RequestURI)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
