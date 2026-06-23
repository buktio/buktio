package metering

import "context"

// NoOp is the OSS sink: it discards every event, makes no network call, and emits
// no telemetry.
type NoOp struct{}

// NewNoOp returns the OSS metering sink.
func NewNoOp() Sink { return NoOp{} }

func (NoOp) Emit(context.Context, Event) error { return nil }
