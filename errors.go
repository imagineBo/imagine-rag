package imagine

import "fmt"

// Error is returned for all non-2xx responses from the API.
type Error struct {
	StatusCode int    `json:"status_code"`
	Code       string `json:"code"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("imagine: [%s] %s (request_id: %s)", e.Code, e.Message, e.RequestID)
}

// Common error code constants — use these in switch statements.
const (
	ErrCodeUnauthorized    = "unauthorized"
	ErrCodeNotFound        = "not_found"
	ErrCodeFileTooLarge    = "file_too_large"
	ErrCodeUnsupportedType = "unsupported_type"
	ErrCodeQuotaExceeded   = "quota_exceeded"
	ErrCodeRateLimited     = "rate_limited"
	ErrCodeInvalidRequest  = "invalid_request"
	ErrCodeServerError     = "server_error"
)

func IsNotFound(err error) bool      { return hasCode(err, ErrCodeNotFound) }
func IsUnauthorized(err error) bool  { return hasCode(err, ErrCodeUnauthorized) }
func IsQuotaExceeded(err error) bool { return hasCode(err, ErrCodeQuotaExceeded) }
func IsRateLimited(err error) bool   { return hasCode(err, ErrCodeRateLimited) }

func hasCode(err error, code string) bool {
	if e, ok := err.(*Error); ok {
		return e.Code == code
	}
	return false
}
