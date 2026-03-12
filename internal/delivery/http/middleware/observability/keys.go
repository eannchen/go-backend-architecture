package observabilitymw

const (
	keyHTTPRequestMethod  = "http.request.method"
	keyHTTPRoute          = "http.route"
	keyHTTPResponseStatus = "http.response.status_code"
	keyURLPath            = "url.path"
	keyRequestID          = "request.id"
	keyTraceID            = "trace.id"
	keySpanID             = "span.id"
	keyDurationMS         = "duration.ms"
	keyError              = "error"
	keyErrorCauseChain    = "error.cause_chain"
	keyErrorDetails       = "error.details"
	keyTransportCode      = "error.transport.code"
	keyTransportMessage   = "error.transport.message"
)
