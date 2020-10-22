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
	MaxWaitBetweenTrial = 1
)

var proxyTransport = http.Transport{
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

type MyTransport struct {
	timeout int
}

func (t *MyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	var response *http.Response
	var err error

	retries := 0
	st := time.Now()

	for {
		response, err = proxyTransport.RoundTrip(request)
		if err == nil {
			break
		}

		log.Printf("[MyTransport/RoundTrip/%d] err=%v\n", retries, err)

		if strings.Contains(err.Error(), "context canceled") {
			break
		}

		// waiting timeout
		if time.Since(st).Seconds() >= float64(t.timeout) {
			break
		}

		time.Sleep(MaxWaitBetweenTrial * time.Second)
		retries++
	}

	// TODO: create fake response
	//
	// if response == nil {
	// 	response = &http.Response{
	// 		StatusCode: 404,
	// 	}
	// 	err = nil
	// }

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
