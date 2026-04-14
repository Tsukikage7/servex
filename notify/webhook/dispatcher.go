package webhook

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

type dispatcher struct {
	opts dispatcherOptions
}

// NewDispatcher 创建 webhook 投递器.
func NewDispatcher(opts ...DispatcherOption) *dispatcher {
	o := dispatcherOptions{
		timeout:         10 * time.Second,
		signer:          NewHMACSigner(),
		signatureHeader: "X-Webhook-Signature",
		eventTypeHeader: "X-Webhook-Event",
		eventIDHeader:   "X-Webhook-ID",
	}
	for _, opt := range opts {
		opt(&o)
	}
	if o.httpClient == nil {
		o.httpClient = &http.Client{Timeout: o.timeout}
	}
	return &dispatcher{opts: o}
}

// Dispatch 向订阅者投递 webhook 事件.
// 失败时自动重试（最多 3 次，指数退避）.
func (d *dispatcher) Dispatch(ctx context.Context, sub *Subscription, event *Event) error {
	if sub == nil {
		return ErrNilSubscription
	}
	if event == nil {
		return ErrNilEvent
	}
	if sub.URL == "" {
		return ErrEmptyURL
	}

	const maxAttempts = 3
	backoff := 500 * time.Millisecond

	var lastErr error
	for attempt := range maxAttempts {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
			}
		}

		err := d.doDispatch(ctx, sub, event)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return lastErr
}

// doDispatch 执行单次 webhook 投递.
func (d *dispatcher) doDispatch(ctx context.Context, sub *Subscription, event *Event) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(event.Payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(d.opts.eventTypeHeader, event.Type)
	req.Header.Set(d.opts.eventIDHeader, event.ID)

	if sub.Secret != "" {
		sig := d.opts.signer.Sign(event.Payload, sub.Secret)
		req.Header.Set(d.opts.signatureHeader, sig)
	}

	resp, err := d.opts.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return ErrDeliveryFailed.WithMessage(fmt.Sprintf("投递失败，状态码 %d", resp.StatusCode))
	}
	return nil
}

// Close 关闭投递器.
func (d *dispatcher) Close() error { return nil }
