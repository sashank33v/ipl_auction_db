package rooms

import (
	"context"
	"errors"
)

var ErrBusy = errors.New("room command queue is full")
var ErrClosed = errors.New("room command manager is closed")

type command struct {
	ctx    context.Context
	run    func(context.Context) error
	result chan error
}
