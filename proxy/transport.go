package proxy

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
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
	upstream string
	mode     string
	timeout  int
}

func (t *MyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	var response *http.Response
	var err error

	retries := 0
	st := time.Now()

	switch t.mode {
	case ProxyModeStoreAndForward:
		// TODO: handle this
	case ProxyModeBypass:
	}

	for {
		response, err = proxyTransport.RoundTrip(request)
		if err == nil {
			break
		}

		log.Printf("[MyTransport/RoundTrip/%d] err=%v\n", retries, err)

		if errors.Is(err, context.Canceled) {
			break
		}

		// waiting timeout
		if time.Since(st).Seconds() >= float64(t.timeout) {
			buf := bytes.NewBuffer([]byte(fmt.Sprintf("timeout %d sec", t.timeout)))
			response, err = http.ReadResponse(bufio.NewReader(buf), request)
			response.StatusCode = http.StatusServiceUnavailable
			response.Status = http.StatusText(response.StatusCode)
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
	//
	//	buf, err := httputil.DumpResponse(resp, true)
	//

	// not disconnected
	if !errors.Is(err, context.Canceled) {
		response.Header.Add("X-Buffy-Elasped", fmt.Sprintf("%.5f sec", time.Since(st).Seconds()))
		response.Header.Add("X-Buffy-Timeout", strconv.Itoa(t.timeout))
		response.Header.Add("X-Buffy-Mode", t.mode)
		response.Header.Add("X-Buffy-Upstream", t.upstream)
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
