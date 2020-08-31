package resource

import (
	"net/http"
)

// Resource represents a HTTP-like resource
type Resource struct {
	OPTIONS   http.Handler
	GET    http.Handler
	HEAD   http.Handler
	POST   http.Handler
	PUT    http.Handler
	DELETE http.Handler
	CONNECT http.Handler
}

var _ http.Handler = &Resource{}

// Handle implements the http.Handler interface and dispatches the methods
// to the resource
func (res Resource) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "OPTIONS":
		if res.OPTIONS != nil {
			res.OPTIONS.ServeHTTP(w, r)
			return
		}
	case "GET":
		if res.GET != nil {
			res.GET.ServeHTTP(w, r)
			return
		}
	case "HEAD":
		if res.HEAD != nil {
			res.HEAD.ServeHTTP(w, r)
			return
		}
		if res.GET != nil {
			// FIXME: remove body
			res.GET.ServeHTTP(w, r)
			return
		}
	case "PUT":
		if res.PUT != nil {
			res.PUT.ServeHTTP(w, r)
			return
		}
	case "DELETE":
		if res.DELETE != nil {
			res.DELETE.ServeHTTP(w, r)
			return
		}
	case "CONNECT":
		if res.CONNECT != nil {
			res.CONNECT.ServeHTTP(w, r)
			return
		}
	}

	http.Error(w, "method "+r.Method+" not implemented", http.StatusNotImplemented)
}

// GET returns a new resource with the GET handler installed
func GET(h http.HandlerFunc) *Resource {
	return &Resource{
		GET: h,
	}
}
