package done

import "sync"

// Instance is a utility for notifications of something being done.
type Instance struct {
	access sync.Mutex
	c      chan struct{}
	closed bool
}

// New returns a new Done.
func New() *Instance {
	return &Instance{
		c: make(chan struct{}),
	}
}

// Wait returns a channel for waiting for done.
func (i *Instance) Wait() <-chan struct{} {
	return i.c
}

func (i *Instance) Done() {
	i.access.Lock()
	defer i.access.Unlock()

	if !i.closed {
		i.closed = true
		close(i.c)
	}
}

// IsDone returns true if Done() is called.
func (i *Instance) IsDone() bool {
	select {
	case <-i.c:
		return true
	default:
		return false
	}
}
