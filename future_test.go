package future

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPromise(t *testing.T) {
	type Input struct {
		V   interface{}
		Err error
	}
	type Expect struct {
		V   interface{}
		Err error
	}
	type Test struct {
		Input  Input
		Expect Expect
	}

	tests := map[string]Test{
		"success": {
			Input: Input{
				1,
				nil,
			},
			Expect: Expect{
				1,
				nil,
			},
		},
		"failure": {
			Input: Input{
				nil,
				errors.New("error"),
			},
			Expect: Expect{
				nil,
				errors.New("error"),
			},
		},
		"both": {
			Input: Input{
				1,
				errors.New("error"),
			},
			Expect: Expect{
				1,
				errors.New("error"),
			},
		},
	}

	for title, test := range tests {
		t.Run(title, func(t *testing.T) {
			assert := assert.New(t)

			p := NewPromise()
			f := p.Future()

			err := p.Complete(test.Input.V, test.Input.Err)
			assert.NoError(err)

			v, err := f.Wait(context.Background())
			assert.Equal(test.Expect.V, v)
			assert.Equal(test.Expect.Err, err)
		})
	}

	t.Run("complete twice", func(t *testing.T) {
		assert := assert.New(t)

		p := NewPromise()

		err := p.Complete(1, errors.New("error"))
		assert.NoError(err)

		err = p.Complete(2, nil)
		assert.Equal(ErrAlreadyDone, err)

		f := p.Future()
		v, err := f.Wait(context.Background())
		assert.Equal(1, v)
		assert.Equal(errors.New("error"), err)
	})
}

func TestGo(t *testing.T) {
	type Input struct {
		F func() (interface{}, error)
	}
	type Expect struct {
		V   interface{}
		Err error
	}
	type Test struct {
		Input  Input
		Expect Expect
	}

	tests := map[string]Test{
		"success": {
			Input: Input{func() (interface{}, error) {
				return 1, nil
			}},
			Expect: Expect{
				1,
				nil,
			},
		},
		"failure": {
			Input: Input{func() (interface{}, error) {
				return nil, errors.New("error")
			}},
			Expect: Expect{
				nil,
				errors.New("error"),
			},
		},
		"both": {
			Input: Input{func() (interface{}, error) {
				return 1, errors.New("error")
			}},
			Expect: Expect{
				1,
				errors.New("error"),
			},
		},
	}

	for title, test := range tests {
		t.Run(title, func(t *testing.T) {
			assert := assert.New(t)

			f := Go(test.Input.F)

			v, err := f.Wait(context.Background())
			assert.Equal(test.Expect.V, v)
			assert.Equal(test.Expect.Err, err)
		})
	}
}

func TestAny(t *testing.T) {
	type Input struct {
		Futures []Future
	}
	type Expect struct {
		V   interface{}
		Err error
	}
	type Test struct {
		Input  Input
		Expect Expect
	}

	err := errors.New("error")
	tests := map[string]Test{
		"success": {
			Input: Input{[]Future{
				Failure(err),
				Failure(err),
				Success(1),
				Failure(err),
			}},
			Expect: Expect{
				1,
				nil,
			},
		},
		"failure": {
			Input: Input{[]Future{
				Failure(err),
				Failure(err),
				Failure(err),
				Failure(err),
			}},
			Expect: Expect{
				nil,
				err,
			},
		},
		"select no error": {
			Input: Input{[]Future{
				Go(func() (interface{}, error) {
					return 1, err
				}),
				Failure(err),
				Go(func() (interface{}, error) {
					return 2, nil
				}),
				Failure(err),
			}},
			Expect: Expect{
				2,
				nil,
			},
		},
		"fastest one": {
			Input: Input{[]Future{
				Go(func() (interface{}, error) {
					time.Sleep(3 * time.Millisecond)
					return 3, nil
				}),
				Failure(err),
				Go(func() (interface{}, error) {
					time.Sleep(1 * time.Millisecond)
					return 1, nil
				}),
				Go(func() (interface{}, error) {
					time.Sleep(2 * time.Millisecond)
					return 2, nil
				}),
			}},
			Expect: Expect{
				1,
				nil,
			},
		},
	}

	for title, test := range tests {
		t.Run(title, func(t *testing.T) {
			assert := assert.New(t)

			f := Any(test.Input.Futures...)

			v, err := f.Wait(context.Background())
			assert.Equal(test.Expect.V, v)
			assert.Equal(test.Expect.Err, err)
		})
	}
}
