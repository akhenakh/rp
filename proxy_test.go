package rp

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/stretchr/testify/require"
)

const (
	b1body            = "b1"
	b2body            = "b2"
	myTestHeader      = "X-Added-Header"
	myTestHeaderValue = "42"
)

func TestProxyLB(t *testing.T) {
	opts := &Options{Logger: log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))}

	ps, clean := setupProxy(t, opts)
	defer clean()

	// starting the proxy http server
	tserv := httptest.NewServer(ps)
	defer tserv.Close()
	req, err := http.NewRequest("GET", tserv.URL, nil)
	require.NoError(t, err)

	// requesting through the proxy
	res, err := tserv.Client().Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, 200, res.StatusCode)

	// reading body
	body, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	require.EqualValues(t, b1body, string(body))

	// requesting through the proxy
	res2, err := tserv.Client().Do(req)
	require.NoError(t, err)
	defer res2.Body.Close()

	require.Equal(t, 200, res.StatusCode)

	// reading body
	body, err = ioutil.ReadAll(res2.Body)
	require.NoError(t, err)
	require.EqualValues(t, b2body, string(body))
}

func TestProxyHeader(t *testing.T) {
	// headers returns by the real servers should be returned to the caller
	ps, clean := setupProxy(t, nil)
	defer clean()

	// starting the proxy http server
	tserv := httptest.NewServer(ps)
	defer tserv.Close()
	req, err := http.NewRequest("GET", tserv.URL, nil)
	require.NoError(t, err)

	// requesting through the proxy
	res, err := tserv.Client().Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	require.Equal(t, 200, res.StatusCode)
	require.Equal(t, myTestHeaderValue, res.Header.Get(myTestHeader))
}

func TestPickBackend(t *testing.T) {
	ps, err := New([]string{}, nil)
	require.NoError(t, err)
	addr, err := ps.PickBackend()
	require.Error(t, err)
	require.Equal(t, addr, "")

	// pick should return round robin backends
	ps, err = New([]string{"1", "2"}, nil)
	require.NoError(t, err)
	for i := 0; i < 100; i++ {
		addr, err = ps.PickBackend()
		require.NoError(t, err)
		require.Equal(t, addr, "1")
		addr, err = ps.PickBackend()
		require.NoError(t, err)
		require.Equal(t, addr, "2")
	}
}

func setupProxy(t *testing.T, opts *Options) (*ProxyServer, func()) {
	// starting 2 backend server
	backend1 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set(myTestHeader, myTestHeaderValue)
		rw.Write([]byte(b1body))
	}))

	backend2 := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set(myTestHeader, myTestHeaderValue)
		rw.Write([]byte(b2body))
	}))

	b1URL, err := url.Parse(backend1.URL)
	require.NoError(t, err)
	b2URL, err := url.Parse(backend2.URL)
	require.NoError(t, err)
	t.Log(backend1.URL)

	ps, err := New([]string{b1URL.Host, b2URL.Host}, opts)
	require.NotNil(t, ps)
	require.NoError(t, err)

	return ps, func() {
		backend1.Close()
		backend2.Close()
	}
}
