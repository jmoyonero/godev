package execxtest

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/jmoyonero/godev/pkg/execx"
)

// ErrKilled is what Wait returns for a fake process stopped with Kill or by
// its context.
var ErrKilled = errors.New("signal: killed")

// Process is a fake background process. It runs until Kill, Exit or the
// cancellation of the context it was started with.
type Process struct {
	Cmd execx.Cmd

	done   chan struct{}
	once   sync.Once
	err    error
	killed atomic.Bool
}

func newProcess(ctx context.Context, c execx.Cmd) *Process {
	p := &Process{Cmd: c, done: make(chan struct{})}
	go func() {
		select {
		case <-ctx.Done():
			p.stop(ErrKilled)
		case <-p.done:
		}
	}()
	return p
}

// Wait implements execx.Process.
func (p *Process) Wait() error {
	<-p.done
	return p.err
}

// Kill implements execx.Process.
func (p *Process) Kill() error {
	p.killed.Store(true)
	p.stop(ErrKilled)
	return nil
}

// Exit makes the process terminate on its own with err (nil for success).
func (p *Process) Exit(err error) {
	p.stop(err)
}

// Killed reports whether Kill was called.
func (p *Process) Killed() bool {
	return p.killed.Load()
}

// Done reports whether the process has terminated for any reason.
func (p *Process) Done() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

func (p *Process) stop(err error) {
	p.once.Do(func() {
		p.err = err
		close(p.done)
	})
}
