package proxy

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	MaxWaitBetweenTrial = 2
)

type MyTransport struct {
}

func (t *MyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	var response *http.Response
	var err error

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 200 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	for i := 0; i < 100; i++ {
		response, err = transport.RoundTrip(request)
		log.Printf("[MyTransport/RoundTrip/%d] res=%#v err=%v\n", i, response, err)
		if err == nil {
			break
		}

		if strings.Contains(err.Error(), "context canceled") {
			break
		}

		time.Sleep(MaxWaitBetweenTrial * time.Second)
	}

	return response, err
}

func drainBody(b io.ReadCloser) (r1, r2 io.ReadCloser, err error) {
	var buf bytes.Buffer
	if _, err = buf.ReadFrom(b); err != nil {
		return nil, b, err
	}
	if err = b.Close(); err != nil {
		return nil, b, err
	}
	return ioutil.NopCloser(&buf), ioutil.NopCloser(bytes.NewReader(buf.Bytes())), nil
}
