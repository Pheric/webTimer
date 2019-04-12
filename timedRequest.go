package main

import (
	"net/http"
	"net/url"
	"sync"
	"time"
)

type TimedRequest struct {
	Page *url.URL
	RequestDuration time.Duration
	ResponseCode int
}

var requests = []TimedRequest{}
var requestsLock sync.Mutex

var client = http.Client {
	Timeout: 500 * time.Millisecond,
}

func MakeRequest(u *url.URL, c chan bool) {
	tr := TimedRequest {
		Page: u,
		ResponseCode: -1,
	}

	req := &http.Request {
		Method: "GET",
		URL: u,
		Header: map[string][]string {
			"Cache-Control": {"no-cache"},
			"From": {"someuser-noreply@webtimer.null"}, // because I can, and maybe one could filter based on this server-side
		},
	}

	startTime := time.Now()
	resp, err := client.Do(req)
	tr.RequestDuration = time.Since(startTime)
	if err == nil && resp != nil {
		tr.ResponseCode = resp.StatusCode

		if resp.Body != nil {
			_ = resp.Body.Close()
		} else {
			// a nil body shouldn't happen
			tr.ResponseCode = -1
		}
	}

	requestsLock.Lock()
	requests = append(requests, tr)
	requestsLock.Unlock()
	c<-true // function finished
}

func GetRequests() []TimedRequest {
	requestsLock.Lock()
	defer requestsLock.Unlock()
	return requests
}

func FreeRequests() {
	requests = []TimedRequest{}
}