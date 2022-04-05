/* SPDX-License-Identifier: Apache-2.0 */
/* Copyright Contributors to the cpackget project. */

package utils_test

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/open-cmsis-pack/cpackget/cmd/utils"
	"github.com/stretchr/testify/assert"
)

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func delHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}

func appendHostToXForwardHeader(header http.Header, host string) {
	// If we aren't the first proxy retain prior
	// X-Forwarded-For information as a comma+space
	// separated list and fold multiple headers into one.
	if prior, ok := header["X-Forwarded-For"]; ok {
		host = strings.Join(prior, ", ") + ", " + host
	}
	header.Set("X-Forwarded-For", host)
}

type proxy struct {
	requestsCount int
	server        *http.Server
	waitGroup     *sync.WaitGroup
}

func (p *proxy) start(t *testing.T, proxyAddress string) {
	assert := assert.New(t)

	p.server = &http.Server{Addr: proxyAddress}

	// Proxy reference: https://gist.github.com/yowu/f7dc34bd4736a65ff28d
	p.server.Handler = http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		p.requestsCount += 1

		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			msg := "unsupported protocal scheme " + req.URL.Scheme
			http.Error(wr, msg, http.StatusBadRequest)
			return
		}

		client := &http.Client{}

		// Request.RequestURI can't be set in client requests.
		// Ref: http://golang.org/src/pkg/net/http/client.go
		req.RequestURI = ""

		delHopHeaders(req.Header)

		if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			appendHostToXForwardHeader(req.Header, clientIP)
		}

		resp, err := client.Do(req)
		if err != nil {
			http.Error(wr, "Server Error", http.StatusInternalServerError)
		}
		defer resp.Body.Close()

		delHopHeaders(resp.Header)
		copyHeader(wr.Header(), resp.Header)

		wr.WriteHeader(resp.StatusCode)
		n, err := io.Copy(wr, resp.Body)
		assert.Nil(err)
		assert.True(n > 0)
	})

	p.waitGroup = &sync.WaitGroup{}
	p.waitGroup.Add(1)

	go func() {
		defer p.waitGroup.Done()

		err := p.server.ListenAndServe()
		if err != http.ErrServerClosed {
			assert.Nil(err)
		}
	}()

	// Wait a little bit until port binding happens
	time.Sleep(2 * time.Millisecond)

	// Make sure proxy is up and running
	conn, err := net.DialTimeout("tcp", proxyAddress, 1*time.Second)
	assert.Nil(err)
	conn.Close()
}

func (p *proxy) stop(t *testing.T) {
	err := p.server.Shutdown(context.TODO())
	assert.Nil(t, err)

	p.waitGroup.Wait()
}

func TestConfigureProxy(t *testing.T) {
	assert := assert.New(t)

	t.Run("test malformed proxy address", func(t *testing.T) {
		err := utils.ConfigureProxy("http://not-a-valid-addres:invalid-port")
		assert.NotNil(err)
	})

	t.Run("test http requests via proxy", func(t *testing.T) {
		expectedContent := "dummy content"
		filePath := "dummy-file"
		defer os.Remove(filePath)

		// Start a testing server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, expectedContent)
		}))

		// Get port
		parsedURL, err := url.Parse(server.URL)
		assert.Nil(err)

		host, port, err2 := net.SplitHostPort(parsedURL.Host)
		assert.Nil(err2)

		portInt, err3 := strconv.Atoi(port)
		assert.Nil(err3)
		if portInt < 65535 {
			portInt += 1
		} else {
			portInt -= 1
		}

		// Start off proxy
		proxyAddress := fmt.Sprintf("%s:%d", host, portInt)
		proxyServer := &proxy{}
		proxyServer.start(t, proxyAddress)

		// Save current HTTPClient
		currHTTPClient := utils.HTTPClient
		currCacheDir := utils.CacheDir

		defer func() {
			// Restore settings
			utils.CacheDir = currCacheDir
			utils.HTTPClient = currHTTPClient
		}()

		utils.CacheDir = ""

		// Configure utils.HTTPClient to use the proxy server
		assert.Nil(utils.ConfigureProxy("http://" + proxyAddress))

		// Make a request
		fileURL := server.URL + "/" + filePath
		outputFile, err4 := utils.DownloadFile(fileURL)
		assert.Nil(err4)

		outputContent, err5 := ioutil.ReadFile(outputFile)
		assert.Nil(err5)

		assert.Equal(expectedContent, string(outputContent))

		proxyServer.stop(t)

		// Make sure only one request went through
		assert.Equal(1, proxyServer.requestsCount)
	})
}
