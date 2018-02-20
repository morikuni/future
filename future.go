package future

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

var (
	ErrAlreadyDone = errors.New("already done")
)

func success(v interface{}) Result {
	return newResult(v, nil)
}

func failure(err error) Result {
	return newResult(nil, err)
}

func newResult(v interface{}, err error) Result {
	return Result{v, err}
}

type Result struct {
	V   interface{}
	Err error
}

type Promise interface {
	Complete(val interface{}, err error) error
	Future() Future
}

func NewPromise() Promise {
	return &promise{
		make([]chan<- Result, 0, 1),
		nil,
		sync.RWMutex{},
	}
}

type promise struct {
	cs     []chan<- Result
	result *Result
	mu     sync.RWMutex
}

func (p *promise) Complete(v interface{}, err error) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.result != nil {
		return ErrAlreadyDone
	}

	r := newResult(v, err)
	p.result = &r
	for _, c := range p.cs {
		c <- r
	}

	return nil
}

func (p *promise) Future() Future {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.result != nil {
		return Fixed(p.result.V, p.result.Err)
	}

	c := make(chan Result, 1)
	p.cs = append(p.cs, c)

	return newFuture(c)
}

func Go(f func() (interface{}, error)) Future {
	p := NewPromise()
	fut := p.Future()
	go func() {
		if err := p.Complete(f()); err != nil {
			// never come here
			panic(fmt.Sprintf("this is a bug of the library: %s", err))
		}
	}()
	return fut
}

type Future interface {
	Wait(ctx context.Context) (interface{}, error)
}

func newFuture(c <-chan Result) Future {
	return &future{c: c}
}

type future struct {
	c    <-chan Result
	done int32
	mu   sync.RWMutex
}

func (f *future) Wait(ctx context.Context) (interface{}, error) {
	if swapped := atomic.CompareAndSwapInt32(&f.done, 0, 1); !swapped {
		return nil, ErrAlreadyDone
	}

	select {
	case r := <-f.c:
		return r.V, r.Err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func Success(v interface{}) Future {
	return Fixed(v, nil)
}

func Failure(err error) Future {
	return Fixed(nil, err)
}

func Fixed(v interface{}, err error) Future {
	return fixedFuture{v, err}
}

type fixedFuture struct {
	v   interface{}
	err error
}

func (f fixedFuture) Wait(context.Context) (interface{}, error) {
	return f.v, f.err
}

func Any(fs ...Future) Future {
	return Go(func() (interface{}, error) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		c := make(chan Result)
		for _, f := range fs {
			go func(f Future) {
				result := newResult(f.Wait(ctx))
				select {
				case c <- result:
				case <-ctx.Done():
				}
			}(f)
		}

		var err error
		for _ = range fs {
			r := <-c
			if r.Err == nil {
				return r.V, nil
			}
			err = r.Err
		}
		return nil, err
	})
}
