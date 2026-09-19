package observabilitymw

const (
	// keyHTTPRequestMethod is the stable OTel attribute for the normalized HTTP method.
	keyHTTPRequestMethod = "http.request.method"
	// keyHTTPRequestMethodOriginal is the stable OTel attribute retained when an unknown method maps to _OTHER.
	keyHTTPRequestMethodOriginal = "http.request.method_original"
	// keyHTTPRoute is the stable OTel attribute for the matched, low-cardinality route template.
	keyHTTPRoute = "http.route"
	// keyHTTPResponseStatusCode is the stable OTel attribute for the native HTTP response status.
	keyHTTPResponseStatusCode = "http.response.status_code"
	// keyURLPath is the stable OTel attribute for the concrete, potentially high-cardinality request path.
	keyURLPath = "url.path"
	// keyURLScheme is the stable OTel attribute for the request's HTTP or HTTPS scheme.
	keyURLScheme = "url.scheme"
	// keyErrorType is the stable OTel failure class; this server uses the 5xx status number as a string.
	keyErrorType = "error.type"
	// keyLogDurationMS is a template log field; span duration and metric histograms record time separately.
	keyLogDurationMS = "duration.ms"
	// keyApplicationErrorCauseChain is a template diagnostic field containing the internal Go error chain.
	keyApplicationErrorCauseChain = "app.error.cause_chain"
	// keyApplicationErrorDetails is a template field containing safe serialized response details.
	keyApplicationErrorDetails = "app.error.details"
	// keyApplicationErrorCode is a template field containing the application or delivery error code.
	keyApplicationErrorCode = "app.error.code"
	// keyApplicationErrorMessage is a template field containing the safe normalized error message.
	keyApplicationErrorMessage = "app.error.message"
	// instrumentationScope identifies spans produced by this HTTP server middleware.
	instrumentationScope = "http-server"
)
