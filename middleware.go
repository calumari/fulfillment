package fulfillment

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"
)

func Logger() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			start := time.Now()
			err := next(c)
			attrs := []any{
				"id", c.ID(),
				"duration", time.Since(start),
			}
			if err != nil {
				slog.Error("processing message", append(attrs, "error", err)...)
			} else {
				slog.Info("processed message", attrs...)
			}
			return err
		}
	}
}

func Recovery() MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					err = fmt.Errorf("panic: %v", r)
					slog.Error("recovered from panic",
						"id", c.ID(),
						"error", err,
						"stack", string(stack),
					)
				}
			}()
			return next(c)
		}
	}
}

func Timeout(d time.Duration) MiddlewareFunc {
	return func(next HandlerFunc) HandlerFunc {
		return func(c *Context) error {
			ctx, cancel := context.WithTimeout(c.Context(), d)
			defer cancel()
			return next(c.WithContext(ctx))
		}
	}
}
