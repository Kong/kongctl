package httpheaders

import "net/http"

const (
	HeaderUserAgent     = "User-Agent"
	HeaderAuthorization = "Authorization"
	HeaderAccept        = "Accept"
	HeaderContentType   = "Content-Type"

	MediaTypeJSON        = "application/json"
	MediaTypeEventStream = "text/event-stream"

	BearerAuthorizationPrefix = "Bearer "
)

// setHeader is the single place that checks for a nil request before setting
// any HTTP header. All public helpers delegate here.
func setHeader(req *http.Request, key, value string) {
	if req == nil {
		return
	}
	req.Header.Set(key, value)
}

// SetUserAgent sets the User-Agent header on the request.
func SetUserAgent(req *http.Request, userAgent string) {
	setHeader(req, HeaderUserAgent, userAgent)
}

// SetBearerAuthorization sets an Authorization header with a Bearer token.
func SetBearerAuthorization(req *http.Request, token string) {
	setHeader(req, HeaderAuthorization, BearerAuthorizationPrefix+token)
}

// SetAccept sets the Accept header on the request.
func SetAccept(req *http.Request, value string) {
	setHeader(req, HeaderAccept, value)
}

// SetAcceptJSON sets the Accept header to application/json.
func SetAcceptJSON(req *http.Request) {
	SetAccept(req, MediaTypeJSON)
}

// SetAcceptEventStream sets the Accept header to text/event-stream.
func SetAcceptEventStream(req *http.Request) {
	SetAccept(req, MediaTypeEventStream)
}

// SetContentType sets the Content-Type header on the request.
func SetContentType(req *http.Request, value string) {
	setHeader(req, HeaderContentType, value)
}

// SetContentTypeJSON sets the Content-Type header to application/json.
func SetContentTypeJSON(req *http.Request) {
	SetContentType(req, MediaTypeJSON)
}
