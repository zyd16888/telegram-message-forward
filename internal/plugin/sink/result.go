package sink

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type DeliveryError struct {
	Kind       FailureKind
	RetryAfter time.Duration
	Err        error
}

func (e *DeliveryError) Error() string { return e.Err.Error() }
func (e *DeliveryError) Unwrap() error { return e.Err }

func PermanentError(format string, args ...any) error {
	return &DeliveryError{Kind: FailurePermanent, Err: fmt.Errorf(format, args...)}
}

func TransientError(err error, retryAfter time.Duration) error {
	return &DeliveryError{Kind: FailureTransient, RetryAfter: retryAfter, Err: err}
}

func ApplyErrorClassification(result *Result, err error) *Result {
	if err == nil {
		return result
	}
	if result == nil {
		result = &Result{Success: false, Error: err.Error()}
	}
	var deliveryErr *DeliveryError
	if errors.As(err, &deliveryErr) {
		result.FailureKind = deliveryErr.Kind
		result.RetryAfter = deliveryErr.RetryAfter
	} else {
		result.FailureKind = FailureTransient
	}
	return result
}

func PermanentResult(summary []byte, message string) *Result {
	return &Result{Success: false, ResponseSummary: summary, Error: message, FailureKind: FailurePermanent}
}

func TransientResult(summary []byte, message string, retryAfter time.Duration) *Result {
	return &Result{Success: false, ResponseSummary: summary, Error: message, FailureKind: FailureTransient, RetryAfter: retryAfter}
}

func HTTPFailure(summary []byte, status int, header http.Header, message string) *Result {
	if status == http.StatusTooManyRequests || status == http.StatusRequestTimeout || status >= 500 {
		return TransientResult(summary, message, ParseRetryAfter(header.Get("Retry-After"), time.Now()))
	}
	return PermanentResult(summary, message)
}

func ParseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}
