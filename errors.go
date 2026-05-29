package imagine

import "fmt"

// Error is returned for every non-2xx response from the API.
//
// You can inspect it directly or use the typed helper functions
// ([IsNotFound], [IsUnauthorized], [IsQuotaExceeded], [IsRateLimited]) to
// avoid matching on the raw code strings yourself.
//
//	if err != nil {
//	    var apiErr *imagine.Error
//	    if errors.As(err, &apiErr) {
//	        fmt.Println("HTTP", apiErr.StatusCode, "–", apiErr.Message)
//	        fmt.Println("request ID:", apiErr.RequestID) // useful for support
//	    }
//	}
type Error struct {
	// StatusCode is the HTTP status code returned by the server (e.g. 401, 404).
	StatusCode int `json:"status_code"`

	// Code is a machine-readable error identifier (one of the ErrCode* constants).
	Code string `json:"code"`

	// Message is a human-readable description of what went wrong.
	Message string `json:"message"`

	// RequestID is the server-assigned trace ID. Include it in support requests.
	RequestID string `json:"request_id"`
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("imagine: [%s] %s (request_id: %s)", e.Code, e.Message, e.RequestID)
}

// Error code constants — use these in switch statements or comparisons instead
// of hard-coding raw strings.
const (
	// ErrCodeUnauthorized is returned when the API key is missing or invalid.
	ErrCodeUnauthorized = "unauthorized"

	// ErrCodeNotFound is returned when the requested resource (KB, file, session)
	// does not exist or has already been deleted.
	ErrCodeNotFound = "not_found"

	// ErrCodeFileTooLarge is returned when an uploaded file exceeds the plan limit.
	ErrCodeFileTooLarge = "file_too_large"

	// ErrCodeUnsupportedType is returned when a file's MIME type is not accepted.
	ErrCodeUnsupportedType = "unsupported_type"

	// ErrCodeQuotaExceeded is returned when the tenant's monthly usage quota is
	// exhausted. Upgrade the plan on the imagine.bo dashboard to continue.
	ErrCodeQuotaExceeded = "quota_exceeded"

	// ErrCodeRateLimited is returned when too many requests are sent in a short
	// window. Back off and retry with exponential delay.
	ErrCodeRateLimited = "rate_limited"

	// ErrCodeInvalidRequest is returned when the request body is missing required
	// fields or contains invalid values.
	ErrCodeInvalidRequest = "invalid_request"

	// ErrCodeServerError is the catch-all for unexpected 5xx responses.
	ErrCodeServerError = "server_error"
)

// IsNotFound reports whether err is an [*Error] with code [ErrCodeNotFound].
//
//	file, err := client.GetFileStatus(ctx, id)
//	if imagine.IsNotFound(err) {
//	    // File was deleted or the ID is wrong.
//	}
func IsNotFound(err error) bool { return hasCode(err, ErrCodeNotFound) }

// IsUnauthorized reports whether err is an [*Error] with code [ErrCodeUnauthorized].
//
// This typically means the API key is wrong, expired, or was revoked.
func IsUnauthorized(err error) bool { return hasCode(err, ErrCodeUnauthorized) }

// IsQuotaExceeded reports whether err is an [*Error] with code [ErrCodeQuotaExceeded].
//
// When true, the tenant's monthly plan limit has been reached.
// Upgrade the plan on the imagine.bo dashboard to continue ingesting or querying.
func IsQuotaExceeded(err error) bool { return hasCode(err, ErrCodeQuotaExceeded) }

// IsRateLimited reports whether err is an [*Error] with code [ErrCodeRateLimited].
//
// When true, the server received too many requests too quickly.
// Implement exponential back-off before retrying.
func IsRateLimited(err error) bool { return hasCode(err, ErrCodeRateLimited) }

// hasCode is the shared implementation for the typed error helpers.
func hasCode(err error, code string) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == code
	}
	return false
}
