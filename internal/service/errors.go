package service

import (
	"errors"
	"net/http"

	"github.com/aws/smithy-go"

	"github.com/buktio/buktio/internal/repository"
	"github.com/buktio/buktio/internal/storage"
	"github.com/buktio/buktio/internal/storage/garage"
)

// Error is a service-level error carrying a stable code + HTTP status, so HTTP
// handlers can map it to the API error envelope.
type Error struct {
	Code    string
	Message string
	HTTP    int
}

func (e *Error) Error() string { return e.Message }

func validationErr(msg string) *Error {
	return &Error{Code: "validation_failed", Message: msg, HTTP: http.StatusBadRequest}
}

func notFoundErr() *Error {
	return &Error{Code: "not_found", Message: "resource not found", HTTP: http.StatusNotFound}
}

func conflictErr(msg string) *Error {
	return &Error{Code: "conflict", Message: msg, HTTP: http.StatusConflict}
}

func storageUnavailableErr(msg string) *Error {
	return &Error{Code: "garage_unavailable", Message: msg, HTTP: http.StatusBadGateway}
}

func unauthorizedErr() *Error {
	return &Error{Code: "unauthenticated", Message: "invalid credentials", HTTP: http.StatusUnauthorized}
}

func unsupportedErr() *Error {
	return &Error{
		Code:    "not_supported_on_backend",
		Message: "this operation is not supported on the bucket's storage backend",
		HTTP:    http.StatusUnprocessableEntity,
	}
}

// mapStorageErr converts a StorageProvider error into a service.Error.
func mapStorageErr(err error) *Error {
	if err == nil {
		return nil
	}
	if errors.Is(err, storage.ErrUnsupported) {
		return unsupportedErr()
	}
	var ae *garage.AdminError
	if errors.As(err, &ae) {
		switch ae.Status {
		case http.StatusConflict:
			return conflictErr("already exists")
		case http.StatusNotFound:
			return notFoundErr()
		case http.StatusBadRequest:
			return validationErr(ae.Body)
		}
	}
	// S3 (smithy) errors from the object plane.
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NoSuchBucket", "NoSuchCORSConfiguration", "NoSuchLifecycleConfiguration":
			return notFoundErr()
		case "AccessDenied":
			return &Error{Code: "forbidden", Message: "access denied", HTTP: http.StatusForbidden}
		case "InvalidRequest", "InvalidArgument", "EntityTooLarge", "KeyTooLongError":
			return validationErr(apiErr.ErrorMessage())
		}
	}
	return storageUnavailableErr("storage backend error: " + err.Error())
}

// mapRepoErr converts a repository error into a service.Error.
func mapRepoErr(err error) *Error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return notFoundErr()
	}
	return &Error{Code: "internal_error", Message: err.Error(), HTTP: http.StatusInternalServerError}
}
