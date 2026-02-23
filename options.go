package fulfillment

import "time"

type Option func(*Consumer)

func WithWorkers(n int) Option {
	return func(r *Consumer) {
		r.workers = max(1, n)
	}
}

func WithVisibilityTimeout(timeout time.Duration) Option {
	return func(r *Consumer) {
		r.visibilityTimeout = timeout
	}
}

func WithHeartbeat(interval time.Duration) Option {
	return func(r *Consumer) {
		r.heartbeatInterval = interval
	}
}

func WithBatchSize(n int32) Option {
	return func(r *Consumer) {
		if n > 0 && n <= 10 {
			r.batchSize = n
		}
	}
}

func WithWaitTime(seconds int32) Option {
	return func(r *Consumer) {
		if seconds >= 0 && seconds <= 20 {
			r.waitTime = seconds
		}
	}
}

func WithDeleteQueueSize(n int) Option {
	return func(r *Consumer) {
		if n > 0 {
			r.deleteQueueSize = n
		}
	}
}

func WithFlushInterval(d time.Duration) Option {
	return func(r *Consumer) {
		if d > 0 {
			r.flushInterval = d
		}
	}
}
