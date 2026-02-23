package fulfillment

import "time"

// EventStatus represents the outcome of a processed message.
type EventStatus string

const (
	EventStatusSuccess EventStatus = "success"
	EventStatusError   EventStatus = "error"
)

// Monitor is the interface for collecting consumer metrics. Implement this
// interface to integrate with any metrics backend (OTel, StatsD, Prometheus,
// etc.) and pass it to the consumer via WithMonitor.
type Monitor interface {
	// RecordMessage is called after each message is processed by a handler.
	RecordMessage(startTime, endTime time.Time, status EventStatus)
	// RecordDelete is called after each delete batch is flushed.
	RecordDelete(succeeded, failed int)
	// RecordHeartbeat is called after each visibility-timeout extension batch.
	RecordHeartbeat(succeeded, failed int)
	// RecordPoll is called after each successful ReceiveMessage call.
	RecordPoll(count int)
}

type noopMonitor struct{}

func (noopMonitor) RecordMessage(_, _ time.Time, _ EventStatus) {}
func (noopMonitor) RecordDelete(_, _ int)                       {}
func (noopMonitor) RecordHeartbeat(_, _ int)                    {}
func (noopMonitor) RecordPoll(_ int)                            {}

func NewNoopMonitor() Monitor { return noopMonitor{} }
