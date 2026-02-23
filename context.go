package fulfillment

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type Context struct {
	ctx     context.Context
	Message *types.Message
}

func NewContext(ctx context.Context, msg *types.Message) *Context {
	return &Context{ctx: ctx, Message: msg}
}

func (c *Context) Context() context.Context {
	return c.ctx
}

func (c *Context) WithContext(ctx context.Context) *Context {
	return &Context{ctx: ctx, Message: c.Message}
}

func (c *Context) Body() string {
	if c.Message.Body == nil {
		return ""
	}
	return *c.Message.Body
}

func (c *Context) ID() string {
	if c.Message.MessageId == nil {
		return ""
	}
	return *c.Message.MessageId
}

func (c *Context) Attribute(key string) string {
	return c.Message.Attributes[key]
}

func (c *Context) MessageAttribute(key string) string {
	if attr, ok := c.Message.MessageAttributes[key]; ok && attr.StringValue != nil {
		return *attr.StringValue
	}
	return ""
}
