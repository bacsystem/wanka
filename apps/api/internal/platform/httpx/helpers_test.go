package httpx_test

import (
	"context"
	"net"
	"time"
)

type netDialer struct{}

func (netDialer) dial(addr string) (net.Conn, error) {
	return net.DialTimeout("tcp", addr, time.Second)
}

func timeNow() time.Time { return time.Now() }

// mergeValues devuelve un contexto con los valores de base y la cancelación de done.
func mergeValues(base, done context.Context) context.Context {
	ctx, cancel := context.WithCancelCause(base)
	if done.Err() != nil {
		cancel(done.Err())
	} else {
		context.AfterFunc(done, func() { cancel(done.Err()) })
	}
	return ctx
}
