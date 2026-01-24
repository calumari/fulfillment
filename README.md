# fulfillment

> **Work in Progress** - This library is under active development and the API may change.

A gorilla/mux-style SQS consumer library for Go with support for middleware, handler routing based on message attributes, automatic message visibility extension (heartbeat), and more.

## Installation

```bash
go get github.com/calumari/fulfillment
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/calumari/fulfillment"
)

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(err)
	}

	client := sqs.NewFromConfig(cfg)
	queueURL := "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue"

	// Create consumer with options
	c := fulfillment.NewConsumer(client, queueURL,
		fulfillment.WithWorkers(5),
		fulfillment.WithVisibilityTimeout(30),
		fulfillment.WithHeartbeat(10*time.Second),
	)

	// Global middleware
	c.Use(fulfillment.Recovery(), fulfillment.Logger())

	// Route messages by attributes
	c.HandleFunc(handleOrder).MessageAttribute("event_type", "order.created")
	c.HandleFunc(handlePayment).MessageAttribute("event_type", "payment.completed")

	// Start consuming messages
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := c.Start(ctx); err != nil {
		panic(err)
	}
}

func handleOrder(c *fulfillment.Context) error {
	fmt.Printf("Order: %s\n", c.Body())
	return nil // Automatically deleted on success
}

func handlePayment(c *fulfillment.Context) error {
	fmt.Printf("Payment: %s\n", c.Body())
	return fmt.Errorf("failed to process payment") // Message will be retried or moved to DLQ (if configured)
}
```
