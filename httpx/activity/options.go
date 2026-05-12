package activity

import (
	"time"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Option 配置选项函数.
type Option func(*options)

type options struct {
	store        Store
	producer     Producer
	extractor    UserIDExtractor
	logger       logger.Logger
	topic        string        // Kafka topic
	asyncMode    bool          // 异步模式
	onlineTTL    time.Duration // 在线状态 TTL
	dedupeWindow time.Duration // 去重窗口
	sampleRate   float64       // 采样率 (0.0-1.0)
}

func defaultOptions() *options {
	return &options{
		topic:        "user_activity_events",
		asyncMode:    true,
		onlineTTL:    5 * time.Minute,
		dedupeWindow: 30 * time.Second,
		sampleRate:   1.0,
	}
}

// WithStore 设置存储后端.
func WithStore(store Store) Option {
	return func(o *options) {
		o.store = store
	}
}

// WithProducer 设置消息生产者.
func WithProducer(producer Producer) Option {
	return func(o *options) {
		o.producer = producer
	}
}

// WithUserIDExtractor 设置用户 ID 提取器.
func WithUserIDExtractor(extractor UserIDExtractor) Option {
	return func(o *options) {
		o.extractor = extractor
	}
}

// WithLogger 设置日志记录器.
func WithLogger(log logger.Logger) Option {
	return func(o *options) {
		o.logger = logger.WithComponent(log, "Activity")
	}
}

// WithTopic 设置 Kafka topic.
func WithTopic(topic string) Option {
	return func(o *options) {
		o.topic = topic
	}
}

// WithAsyncMode 设置异步模式.
// 异步模式下，事件发送到消息队列后立即返回，不等待确认.
// 默认启用.
func WithAsyncMode(enabled bool) Option {
	return func(o *options) {
		o.asyncMode = enabled
	}
}

// WithOnlineTTL 设置在线状态 TTL.
// 用户在此时间内有活动被认为在线.
// 默认 5 分钟.
func WithOnlineTTL(ttl time.Duration) Option {
	return func(o *options) {
		o.onlineTTL = ttl
	}
}

// WithDedupeWindow 设置去重窗口.
// 同一用户在此时间窗口内的多次请求只记录一次.
// 用于减少存储压力.
// 默认 30 秒.
func WithDedupeWindow(window time.Duration) Option {
	return func(o *options) {
		o.dedupeWindow = window
	}
}

// WithSampleRate 设置采样率.
// 1.0 表示记录所有请求，0.1 表示只记录 10%.
// 用于高流量场景降低存储压力.
// 默认 1.0.
func WithSampleRate(rate float64) Option {
	return func(o *options) {
		if rate > 0 && rate <= 1 {
			o.sampleRate = rate
		}
	}
}
