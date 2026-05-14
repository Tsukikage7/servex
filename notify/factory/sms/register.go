// Package sms registers the SMS notify factory.
package sms

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/notify"
	"github.com/Tsukikage7/servex/v2/notify/factory"
	"github.com/Tsukikage7/servex/v2/notify/sms"
	"github.com/Tsukikage7/servex/v2/observability/logger"
)

func init() {
	factory.MustRegisterSender("sms", func(cfg *factory.Config, log logger.Logger) (notify.Sender, error) {
		smsConfig := cfg.SMS
		var provider sms.Provider
		switch smsConfig.Provider {
		case "aliyun":
			if smsConfig.Aliyun == nil {
				smsConfig.Aliyun = &factory.AliyunSMSConfig{}
			}
			provider = sms.NewAliyunProvider(sms.AliyunConfig{
				AccessKeyID:     smsConfig.Aliyun.AccessKeyID,
				AccessKeySecret: smsConfig.Aliyun.AccessKeySecret,
				SignName:        smsConfig.SignName,
				Endpoint:        smsConfig.Aliyun.Endpoint,
			})
		case "tencent":
			if smsConfig.Tencent == nil {
				smsConfig.Tencent = &factory.TencentSMSConfig{}
			}
			provider = sms.NewTencentProvider(sms.TencentConfig{
				SecretID:  smsConfig.Tencent.SecretID,
				SecretKey: smsConfig.Tencent.SecretKey,
				AppID:     smsConfig.Tencent.AppID,
				SignName:  smsConfig.SignName,
				Endpoint:  smsConfig.Tencent.Endpoint,
			})
		default:
			return nil, fmt.Errorf("不支持的 SMS provider %q", smsConfig.Provider)
		}

		opts := []sms.Option{sms.WithSignName(smsConfig.SignName)}
		if log != nil {
			opts = append(opts, sms.WithLogger(log))
		}
		return sms.NewSender(provider, opts...)
	})
}
