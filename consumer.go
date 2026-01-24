package fulfillment

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type SQSClient interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessageBatch(ctx context.Context, params *sqs.DeleteMessageBatchInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageBatchOutput, error)
	ChangeMessageVisibilityBatch(ctx context.Context, params *sqs.ChangeMessageVisibilityBatchInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityBatchOutput, error)
}

type HandlerFunc func(*Context) error

type MiddlewareFunc func(HandlerFunc) HandlerFunc

type Consumer struct {
	client     SQSClient
	url        string
	routes     []*Route
	middleware []MiddlewareFunc

	workers           int
	visibilityTimeout int32
	heartbeatInterval time.Duration
	batchSize         int32
	waitTime          int32
	deleteQueueSize   int
	flushInterval     time.Duration

	deleteQueue    chan *types.Message
	heartbeatQueue *syncMap[string, *types.Message]
}

const (
	maxBatchSize = 10 // AWS SQS limit
)

func NewConsumer(client SQSClient, queueURL string, opts ...Option) *Consumer {
	c := &Consumer{
		client:            client,
		url:               queueURL,
		workers:           1,
		visibilityTimeout: 30,
		batchSize:         10,
		waitTime:          20,
		deleteQueueSize:   100,
		flushInterval:     time.Second,
		heartbeatQueue:    newSyncMap[string, *types.Message](),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Consumer) Use(mw ...MiddlewareFunc) *Consumer {
	c.middleware = append(c.middleware, mw...)
	return c
}

func (c *Consumer) HandleFunc(h HandlerFunc) *Route {
	route := &Route{handler: h}
	c.routes = append(c.routes, route)
	return route
}

func (c *Consumer) Start(ctx context.Context) error {
	var wg sync.WaitGroup
	msgs := make(chan types.Message, c.workers)
	c.deleteQueue = make(chan *types.Message, c.deleteQueueSize)

	wg.Go(func() {
		c.batchDeleteWorker(ctx)
	})

	if c.heartbeatInterval > 0 {
		wg.Go(func() {
			c.batchHeartbeatWorker(ctx)
		})
	}

	for i := 0; i < c.workers; i++ {
		wg.Go(func() {
			for msg := range msgs {
				c.process(ctx, msg)
			}
		})
	}

	wg.Go(func() {
		defer close(msgs)
		c.poll(ctx, msgs)
	})

	<-ctx.Done()
	close(c.deleteQueue)
	wg.Wait()

	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	return ctx.Err()
}

func (c *Consumer) poll(ctx context.Context, msgs chan<- types.Message) {
	const (
		minBackoff = 100 * time.Millisecond
		maxBackoff = 30 * time.Second
	)
	backoff := minBackoff

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		out, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:              &c.url,
			MaxNumberOfMessages:   c.batchSize,
			WaitTimeSeconds:       c.waitTime,
			VisibilityTimeout:     c.visibilityTimeout,
			MessageAttributeNames: []string{"All"},
			AttributeNames:        []types.QueueAttributeName{types.QueueAttributeNameAll},
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Error("polling messages", "error", err, "backoff", backoff)

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}

			backoff = min(backoff*2, maxBackoff)
			continue
		}

		backoff = minBackoff

		for _, msg := range out.Messages {
			select {
			case msgs <- msg:
			case <-ctx.Done():
				return
			}
		}
	}
}

func (r *Consumer) process(ctx context.Context, msg types.Message) {
	c := &Context{
		ctx:     ctx,
		Message: &msg,
	}

	var route *Route
	for _, rt := range r.routes {
		if rt.matches(c) {
			route = rt
			break
		}
	}

	if route == nil {
		return
	}

	h := route.handler
	for i := len(route.middleware) - 1; i >= 0; i-- {
		h = route.middleware[i](h)
	}
	for i := len(r.middleware) - 1; i >= 0; i-- {
		h = r.middleware[i](h)
	}

	if r.heartbeatInterval > 0 && msg.MessageId != nil {
		r.heartbeatQueue.Store(*msg.MessageId, &msg)
		defer r.heartbeatQueue.Delete(*msg.MessageId)
	}

	err := h(c)
	if err != nil {
		return
	}

	r.queueDelete(&msg)
}

func (c *Consumer) queueDelete(msg *types.Message) {
	if msg.ReceiptHandle == nil {
		return
	}
	select {
	case c.deleteQueue <- msg:
	default:
		slog.Warn("delete queue full, message will retry", "id", *msg.MessageId)
	}
}

func (c *Consumer) batchDeleteWorker(ctx context.Context) {
	batch := make([]*types.Message, 0, maxBatchSize)
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	flush := func(ctx context.Context) {
		if len(batch) == 0 {
			return
		}

		for _, msgs := range chunk(batch, maxBatchSize) {
			entries := make([]types.DeleteMessageBatchRequestEntry, 0, len(msgs))
			for _, msg := range msgs {
				if msg.MessageId != nil && msg.ReceiptHandle != nil {
					entries = append(entries, types.DeleteMessageBatchRequestEntry{
						Id:            msg.MessageId,
						ReceiptHandle: msg.ReceiptHandle,
					})
				}
			}

			if len(entries) == 0 {
				continue
			}

			result, err := c.client.DeleteMessageBatch(ctx, &sqs.DeleteMessageBatchInput{
				QueueUrl: &c.url,
				Entries:  entries,
			})
			if err != nil {
				slog.Error("deleting messages", "error", err, "count", len(entries))
				continue
			}
			for _, f := range result.Failed {
				slog.Warn("deleting message", "id", *f.Id, "code", *f.Code, "error", *f.Message)
			}
		}

		batch = batch[:0]
	}

	for {
		select {
		case msg, ok := <-c.deleteQueue:
			if !ok {
				flush(context.Background())
				return
			}
			batch = append(batch, msg)
			if len(batch) >= maxBatchSize {
				flush(ctx)
			}
		case <-ticker.C:
			flush(ctx)
		}
	}
}

func (c *Consumer) batchHeartbeatWorker(ctx context.Context) {
	ticker := time.NewTicker(c.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			messages := c.heartbeatQueue.Values()

			if len(messages) == 0 {
				continue
			}

			for _, msgs := range chunk(messages, maxBatchSize) {
				entries := make([]types.ChangeMessageVisibilityBatchRequestEntry, 0, len(msgs))
				for _, msg := range msgs {
					if msg.MessageId != nil && msg.ReceiptHandle != nil {
						entries = append(entries, types.ChangeMessageVisibilityBatchRequestEntry{
							Id:                msg.MessageId,
							ReceiptHandle:     msg.ReceiptHandle,
							VisibilityTimeout: c.visibilityTimeout,
						})
					}
				}

				if len(entries) == 0 {
					continue
				}

				result, err := c.client.ChangeMessageVisibilityBatch(ctx, &sqs.ChangeMessageVisibilityBatchInput{
					QueueUrl: &c.url,
					Entries:  entries,
				})
				if err != nil {
					slog.Error("extending visibility timeout", "error", err, "count", len(entries))
					continue
				}
				for _, f := range result.Failed {
					slog.Warn("extending visibility timeout", "id", *f.Id, "code", *f.Code, "error", *f.Message)
				}
			}
		}
	}
}
