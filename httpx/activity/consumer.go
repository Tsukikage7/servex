package activity

import (
	"context"
	"encoding/json"
	"fmt"
)

// Consumer 活跃事件消费者.
type Consumer struct {
	tracker *Tracker
}

// NewConsumer 创建消费者.
func NewConsumer(tracker *Tracker) *Consumer {
	return &Consumer{tracker: tracker}
}

// HandleMessage 处理消息.
// 此方法应该被 Kafka/RabbitMQ 消费者调用.
func (c *Consumer) HandleMessage(ctx context.Context, data []byte) error {
	event, err := UnmarshalEvent(data)
	if err != nil {
		return fmt.Errorf("activity: failed to unmarshal event: %w", err)
	}

	// 同步写入存储
	if c.tracker.opts.store != nil {
		if err := c.tracker.opts.store.SetLastActive(ctx, event.UserID, event); err != nil {
			return fmt.Errorf("activity: failed to set last active: %w", err)
		}
		if err := c.tracker.opts.store.SetOnline(ctx, event.UserID, c.tracker.opts.onlineTTL); err != nil {
			return fmt.Errorf("activity: failed to set online: %w", err)
		}
	}

	return nil
}

// BatchHandler 批量消息处理器.
type BatchHandler struct {
	tracker   *Tracker
	batchSize int
}

// NewBatchHandler 创建批量处理器.
func NewBatchHandler(tracker *Tracker, batchSize int) *BatchHandler {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &BatchHandler{
		tracker:   tracker,
		batchSize: batchSize,
	}
}

// HandleBatch 批量处理消息.
func (h *BatchHandler) HandleBatch(ctx context.Context, messages [][]byte) error {
	events := make([]*Event, 0, len(messages))

	for _, data := range messages {
		var event Event
		if err := json.Unmarshal(data, &event); err != nil {
			continue // 跳过无法解析的消息
		}
		events = append(events, &event)
	}

	// 按用户聚合，只保留最新的事件
	latestEvents := make(map[string]*Event)
	for _, event := range events {
		existing, ok := latestEvents[event.UserID]
		if !ok || event.Timestamp.After(existing.Timestamp) {
			latestEvents[event.UserID] = event
		}
	}

	// 批量写入
	if h.tracker.opts.store == nil {
		return nil
	}

	for userID, event := range latestEvents {
		if err := h.tracker.opts.store.SetLastActive(ctx, userID, event); err != nil {
			// 记录错误但继续处理
			if h.tracker.opts.logger != nil {
				h.tracker.opts.logger.Error("[Activity] 批量设置最后活跃时间失败",
					"user_id", userID,
					"error", err,
				)
			}
		}
		if err := h.tracker.opts.store.SetOnline(ctx, userID, h.tracker.opts.onlineTTL); err != nil {
			if h.tracker.opts.logger != nil {
				h.tracker.opts.logger.Error("[Activity] 批量设置在线状态失败",
					"user_id", userID,
					"error", err,
				)
			}
		}
	}

	return nil
}
