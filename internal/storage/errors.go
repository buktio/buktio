package storage

import "errors"

// ErrUnsupported is returned by a provider whose backend cannot perform an
// operation (e.g. a generic-S3 backend buktio reaches with operator-supplied
// credentials cannot manage access keys or report cluster health). Services and
// the API map it to a clean "not supported on this backend" response; the UI gates
// the control via Capabilities so the error is a backstop, not the primary signal.
var ErrUnsupported = errors.New("storage: operation not supported by this backend")
