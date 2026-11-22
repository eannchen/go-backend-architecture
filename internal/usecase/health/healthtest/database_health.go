package healthtest

import (
	"context"

	usecasehealth "github.com/eannchen/go-backend-architecture/internal/usecase/health"
)

// DatabaseHealth is a configurable double for its parent package's DatabaseHealth contract.
type DatabaseHealth struct {
	PingFunc             func(ctx context.Context) error
	PingCalls            []DatabaseHealthPingCall
	GetServerStatusFunc  func(ctx context.Context) (usecasehealth.DBServerStatus, error)
	GetServerStatusCalls []DatabaseHealthGetServerStatusCall
}

// DatabaseHealthPingCall records one DatabaseHealth.Ping invocation.
type DatabaseHealthPingCall struct {
	Context context.Context
}

func (d *DatabaseHealth) Ping(ctx context.Context) error {
	d.PingCalls = append(d.PingCalls, DatabaseHealthPingCall{Context: ctx})
	fn := d.PingFunc
	if fn == nil {
		panic("unexpected DatabaseHealth.Ping call")
	}
	return fn(ctx)
}

// DatabaseHealthGetServerStatusCall records one DatabaseHealth.GetServerStatus invocation.
type DatabaseHealthGetServerStatusCall struct {
	Context context.Context
}

func (d *DatabaseHealth) GetServerStatus(ctx context.Context) (usecasehealth.DBServerStatus, error) {
	d.GetServerStatusCalls = append(d.GetServerStatusCalls, DatabaseHealthGetServerStatusCall{Context: ctx})
	fn := d.GetServerStatusFunc
	if fn == nil {
		panic("unexpected DatabaseHealth.GetServerStatus call")
	}
	return fn(ctx)
}
