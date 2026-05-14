// Package factory 提供 Config 驱动的 notify.Dispatcher 工厂.
package factory

import (
	"fmt"
	"strings"
	"sync"

	"github.com/Tsukikage7/servex/v2/errors"
	"github.com/Tsukikage7/servex/v2/notify"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// Config 聚合所有通知渠道的配置.
type Config struct {
	DefaultChannel string         `json:"default_channel" yaml:"default_channel"`
	TemplateDir    string         `json:"template_dir"    yaml:"template_dir"`
	Email          *EmailConfig   `json:"email"           yaml:"email"`
	SMS            *SMSConfig     `json:"sms"             yaml:"sms"`
	Webhook        *WebhookConfig `json:"webhook"         yaml:"webhook"`
	Push           *PushConfig    `json:"push"            yaml:"push"`
}

// EmailConfig 邮件发送配置.
type EmailConfig struct {
	Host, Username, Password, From, Name string
	Port                                 int
	TLS                                  bool
}

// SMSConfig 短信发送配置.
type SMSConfig struct {
	Provider string
	SignName string
	Aliyun   *AliyunSMSConfig
	Tencent  *TencentSMSConfig
}

// AliyunSMSConfig 阿里云短信配置.
type AliyunSMSConfig struct{ AccessKeyID, AccessKeySecret, Endpoint string }

// TencentSMSConfig 腾讯云短信配置.
type TencentSMSConfig struct{ SecretID, SecretKey, AppID, Endpoint string }

// WebhookConfig Webhook 发送配置.
type WebhookConfig struct {
	Timeout int
	Retry   int
}

// PushConfig 推送发送配置.
type PushConfig struct {
	Provider string
	FCM      *FCMPushConfig
	APNs     *APNsPushConfig
}

// FCMPushConfig Firebase Cloud Messaging 配置.
type FCMPushConfig struct {
	ProjectID       string
	CredentialsJSON string
}

// APNsPushConfig Apple Push Notification service 配置.
type APNsPushConfig struct {
	BundleID, TeamID, KeyID, KeyFile string
	Production                       bool
}

var (
	errNilConfig           = errors.NewWithKind(70051, "notify.factory.nil_config", "config 不能为空", errors.KindInvalidArgument)
	errEmailSenderFailed   = errors.NewWithKind(70052, "notify.factory.email_sender_failed", "创建 email sender 失败", errors.KindInternal)
	errWebhookSenderFailed = errors.NewWithKind(70053, "notify.factory.webhook_sender_failed", "创建 webhook sender 失败", errors.KindInternal)
	errUnsupportedSMS      = errors.NewWithKind(70054, "notify.factory.unsupported_sms_provider", "不支持的 SMS provider", errors.KindInvalidArgument)
	errUnsupportedPush     = errors.NewWithKind(70055, "notify.factory.unsupported_push_provider", "不支持的 push provider", errors.KindInvalidArgument)
)

// SenderBuilder 根据聚合配置创建通知发送器.
type SenderBuilder func(*Config, logger.Logger) (notify.Sender, error)

var registry = struct {
	sync.RWMutex
	builders map[string]SenderBuilder
}{
	builders: make(map[string]SenderBuilder),
}

// NewDispatcher 根据 Config 创建并配置好 *notify.Dispatcher.
func NewDispatcher(cfg *Config, log logger.Logger) (*notify.Dispatcher, error) {
	if cfg == nil {
		return nil, errNilConfig
	}

	var opts []notify.Option
	if log != nil {
		opts = append(opts, notify.WithLogger(log))
	}
	if cfg.DefaultChannel != "" {
		opts = append(opts, notify.WithDefaultChannel(notify.Channel(cfg.DefaultChannel)))
	}
	if cfg.TemplateDir != "" {
		opts = append(opts, notify.WithTemplateEngine(
			notify.NewTemplateEngine(notify.WithTemplateDir(cfg.TemplateDir)),
		))
	}

	d := notify.NewDispatcher(opts...)

	if cfg.Email != nil {
		s, err := buildSender("email", cfg, log)
		if err != nil {
			return nil, errEmailSenderFailed.WithCause(err)
		}
		d.Register(s)
	}

	if cfg.SMS != nil {
		s, err := buildSender("sms", cfg, log)
		if err != nil {
			return nil, err
		}
		d.Register(s)
	}

	if cfg.Webhook != nil {
		s, err := buildSender("webhook", cfg, log)
		if err != nil {
			return nil, errWebhookSenderFailed.WithCause(err)
		}
		d.Register(s)
	}

	if cfg.Push != nil {
		s, err := buildSender("push", cfg, log)
		if err != nil {
			return nil, err
		}
		d.Register(s)
	}

	return d, nil
}

// RegisterSender 注册通知发送器构造器.
//
// 具体渠道子包应在 init 中调用 RegisterSender。根 factory 包不直接导入
// email/sms/webhook/push provider，避免业务仅使用部分渠道时被动拉入所有依赖.
func RegisterSender(channel string, builder SenderBuilder) error {
	channel = normalizeChannel(channel)
	if channel == "" {
		return fmt.Errorf("notify/factory: channel 不能为空")
	}
	if builder == nil {
		return fmt.Errorf("notify/factory: sender builder 不能为空")
	}

	registry.Lock()
	defer registry.Unlock()
	if _, exists := registry.builders[channel]; exists {
		return fmt.Errorf("notify/factory: channel %q 已注册", channel)
	}
	registry.builders[channel] = builder
	return nil
}

// MustRegisterSender 注册通知发送器构造器，失败时 panic.
func MustRegisterSender(channel string, builder SenderBuilder) {
	if err := RegisterSender(channel, builder); err != nil {
		panic(err)
	}
}

func buildSender(channel string, cfg *Config, log logger.Logger) (notify.Sender, error) {
	registry.RLock()
	builder, ok := registry.builders[normalizeChannel(channel)]
	registry.RUnlock()
	if !ok {
		switch channel {
		case "sms":
			return nil, errUnsupportedSMS.WithMessage("SMS sender 未注册")
		case "push":
			return nil, errUnsupportedPush.WithMessage("push sender 未注册")
		default:
			return nil, fmt.Errorf("notify/factory: sender channel %q 未注册", channel)
		}
	}
	return builder(cfg, log)
}

func normalizeChannel(channel string) string {
	return strings.ToLower(strings.TrimSpace(channel))
}
