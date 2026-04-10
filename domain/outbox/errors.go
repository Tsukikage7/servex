package outbox

import "errors"

var (
	// ErrNilStore Store 为空.
	ErrNilStore = errors.New("outbox: Store 为空")

	// ErrNilProducer Producer 为空.
	ErrNilProducer = errors.New("outbox: Producer 为空")

	// ErrRelayAlreadyRunning 中继器已运行.
	ErrRelayAlreadyRunning = errors.New("outbox: 中继器已运行")

	// ErrRelayNotRunning 中继器未运行.
	ErrRelayNotRunning = errors.New("outbox: 中继器未运行")

	// ErrEmptyTopic 消息主题为空.
	ErrEmptyTopic = errors.New("outbox: 消息主题为空")

	// ErrEmptyValue 消息内容为空.
	ErrEmptyValue = errors.New("outbox: 消息内容为空")

	// ErrNilDB 数据库实例为空.
	ErrNilDB = errors.New("outbox: 数据库实例为空")

	// ErrMaxRetriesExceeded 消息已达最大重试次数.
	ErrMaxRetriesExceeded = errors.New("outbox: 消息已达最大重试次数")
)
