package state

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	restate "github.com/restatedev/sdk-go"
	"github.com/restatedev/sdk-go/generated/proto/protocol"
	"github.com/restatedev/sdk-go/internal/wire"
)

var (
	_ restate.Service = (*serviceProxy)(nil)
	_ restate.Object  = (*serviceProxy)(nil)
	_ restate.Call    = (*serviceCall)(nil)
)

// service proxy only works as an extension to context
// to implement other services function calls
type serviceProxy struct {
	*Context
	service string
	key     string
}

func (c *serviceProxy) Method(fn string) restate.Call {
	return &serviceCall{
		Context: c.Context,
		service: c.service,
		key:     c.key,
		method:  fn,
	}
}

type serviceCall struct {
	*Context
	service string
	key     string
	method  string
}

type responseFuture struct {
	ctx context.Context
	err error
	msg *wire.CallEntryMessage
}

// Do makes a call and wait for the response
func (c *serviceCall) Request(input any) restate.ResponseFuture {
	if msg, err := c.machine.doDynCall(c.service, c.key, c.method, input); err != nil {
		return &responseFuture{ctx: c.ctx, err: err}
	} else {
		return &responseFuture{ctx: c.ctx, msg: msg}
	}
}

// Send runs a call in the background after delay duration
func (c *serviceCall) Send(body any, delay time.Duration) error {
	return c.machine.sendCall(c.service, c.key, c.method, body, delay)
}

func (r *responseFuture) Err() error {
	return r.err
}

func (r *responseFuture) Response(output any) error {
	if r.err != nil {
		return r.err
	}

	if err := r.msg.Await(r.ctx); err != nil {
		return err
	}

	var bytes []byte
	switch result := r.msg.Result.(type) {
	case *protocol.CallEntryMessage_Failure:
		return ErrorFromFailure(result.Failure)
	case *protocol.CallEntryMessage_Value:
		bytes = result.Value
	default:
		return restate.TerminalError(fmt.Errorf("sync call had invalid result: %v", r.msg.Result), restate.ErrProtocolViolation)

	}

	if err := json.Unmarshal(bytes, output); err != nil {
		// TODO: is this should be a terminal error or not?
		return restate.TerminalError(fmt.Errorf("failed to decode response (%s): %w", string(bytes), err))
	}

	return nil
}

func (m *Machine) doDynCall(service, key, method string, input any) (*wire.CallEntryMessage, error) {
	params, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	return m.doCall(service, key, method, params)
}

func (m *Machine) doCall(service, key, method string, params []byte) (*wire.CallEntryMessage, error) {
	m.log.Debug().Str("service", service).Str("method", method).Str("key", key).Msg("executing sync call")

	return replayOrNew(
		m,
		func(entry *wire.CallEntryMessage) (*wire.CallEntryMessage, error) {
			if entry.ServiceName != service ||
				entry.Key != key ||
				entry.HandlerName != method ||
				!bytes.Equal(entry.Parameter, params) {
				return nil, errEntryMismatch
			}

			return entry, nil
		}, func() (*wire.CallEntryMessage, error) {
			return m._doCall(service, key, method, params)
		})
}

func (m *Machine) _doCall(service, key, method string, params []byte) (*wire.CallEntryMessage, error) {
	msg := &wire.CallEntryMessage{
		CallEntryMessage: protocol.CallEntryMessage{
			ServiceName: service,
			HandlerName: method,
			Parameter:   params,
			Key:         key,
		},
	}
	if err := m.Write(msg); err != nil {
		return nil, fmt.Errorf("failed to send request message: %w", err)
	}

	return msg, nil
}

func (c *Machine) sendCall(service, key, method string, body any, delay time.Duration) error {
	c.log.Debug().Str("service", service).Str("method", method).Str("key", key).Msg("executing async call")

	params, err := json.Marshal(body)
	if err != nil {
		return err
	}

	_, err = replayOrNew(
		c,
		func(entry *wire.OneWayCallEntryMessage) (restate.Void, error) {
			if entry.ServiceName != service ||
				entry.Key != key ||
				entry.HandlerName != method ||
				!bytes.Equal(entry.Parameter, params) {
				return restate.Void{}, errEntryMismatch
			}

			return restate.Void{}, nil
		},
		func() (restate.Void, error) {
			return restate.Void{}, c._sendCall(service, key, method, params, delay)
		},
	)

	return err
}

func (c *Machine) _sendCall(service, key, method string, params []byte, delay time.Duration) error {
	var invokeTime uint64
	if delay != 0 {
		invokeTime = uint64(time.Now().Add(delay).UnixMilli())
	}

	err := c.Write(&wire.OneWayCallEntryMessage{
		OneWayCallEntryMessage: protocol.OneWayCallEntryMessage{
			ServiceName: service,
			HandlerName: method,
			Parameter:   params,
			Key:         key,
			InvokeTime:  invokeTime,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to send request message: %w", err)
	}

	return nil
}

func ErrorFromFailure(failure *protocol.Failure) error {
	return restate.TerminalError(fmt.Errorf(failure.Message), restate.Code(failure.Code))
}
