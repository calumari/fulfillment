package fulfillment

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type Context struct {
	ctx     context.Context
	message *types.Message
}

func NewContext(ctx context.Context, msg *types.Message) *Context {
	return &Context{ctx: ctx, message: msg}
}

func (c *Context) Context() context.Context {
	return c.ctx
}

func (c *Context) WithContext(ctx context.Context) *Context {
	return &Context{ctx: ctx, message: c.message}
}

func (c *Context) Message() *types.Message {
	return c.message
}

func (c *Context) Body() string {
	if c.message.Body == nil {
		return ""
	}
	return *c.message.Body
}

func (c *Context) ID() string {
	if c.message.MessageId == nil {
		return ""
	}
	return *c.message.MessageId
}

func (c *Context) Attribute(key string) string {
	return c.message.Attributes[key]
}

func (c *Context) MessageAttribute(key string) string {
	if attr, ok := c.message.MessageAttributes[key]; ok && attr.StringValue != nil {
		return *attr.StringValue
	}
	return ""
}
