package multiclient

import (
	"net/http"
	"net/url"
	"sync"
	"fmt"
)

type MultiClient struct {
	sync.Mutex
	http.Client
	backends map[string]*url.URL
}

func (c *MultiClient) AddBackend(rawurl string) (err error) {
	if c.backends == nil {
		c.backends = make(map[string]*url.URL)
	}
	c.backends[rawurl], err = url.Parse(rawurl)
	return err
}

func (c *MultiClient) RemoveBackend(rawurl string) {
	delete(c.backends, rawurl)
}

// TODO: make it parallel
// TODO: add timeout
func (c *MultiClient) Do(req *http.Request) (resp *http.Response, err error) {
	for _, backendURL := range c.backends {
		var (
			c2 http.Client
			r http.Request
		)
		c2 = c.Client
		r = *req
		r.Response = nil
		u := backendURL.ResolveReference(r.URL)
		u.Host = backendURL.Host
		u.Scheme = backendURL.Scheme
		r.URL = u
		r.Host = u.Host
		r.Header.Set("Host", u.Host)

		resp, err = c2.Do(&r)

		fmt.Println("request=", r, "resp=", resp, "err=", err)

		if err == nil && resp.StatusCode < 400 {
			break
		}
	}
	return
}

func (c *MultiClient) Get(url string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	return c.Do(req)
}

