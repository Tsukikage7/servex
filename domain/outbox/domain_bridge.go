package outbox

import (
	"context"

	"github.com/Tsukikage7/servex/v2/domain"
	"github.com/Tsukikage7/servex/v2/messaging/pubsub"
)

// OutboxPublisher 通过 Outbox 发布领域事件，保证事务一致性.
// 典型用法：
//
//	publisher := outbox.NewOutboxPublisher(store, domain.NewJSONEventConverter())
//	store.WithTx(ctx, func(txCtx context.Context) error {
//	    // 业务操作...
//	    return publisher.Publish(txCtx, event1, event2)
//	})
type OutboxPublisher struct {
	store     Store
	converter domain.EventConverter
}

// NewOutboxPublisher 创建 OutboxPublisher.
func NewOutboxPublisher(store Store, converter domain.EventConverter) *OutboxPublisher {
	return &OutboxPublisher{
		store:     store,
		converter: converter,
	}
}

// Publish 将领域事件转换为 Outbox 消息并保存.
// 若 ctx 中注入了事务（通过 outbox.InjectTx），则在该事务中保存，
// 保证与业务操作的原子性.
func (p *OutboxPublisher) Publish(ctx context.Context, events ...domain.DomainEvent) error {
	if len(events) == 0 {
		return nil
	}

	msgs := make([]*OutboxMessage, 0, len(events))
	for _, event := range events {
		msg, err := p.converter.Convert(event)
		if err != nil {
			return err
		}
		msgs = append(msgs, NewOutboxMessage(msg))
	}

	return p.store.Save(ctx, msgs...)
}

// 编译期断言 domain.EventConverter 接口已由 domain 包实现.
var _ domain.EventConverter = (*domain.JSONEventConverter)(nil)

// domainConverterAdapter 将 domain.EventConverter 转为 pubsub.Message 的适配器（内部用）.
type domainConverterAdapter struct {
	converter domain.EventConverter
}

func (a *domainConverterAdapter) Convert(event domain.DomainEvent) (*pubsub.Message, error) {
	return a.converter.Convert(event)
}
