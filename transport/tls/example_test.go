package tlsx_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/v2/transport/tls"
)

func ExampleNewTLSConfig_validation() {
	// Nil config returns error.
	_, err := tlsx.NewTLSConfig(nil)
	fmt.Println(err)

	// Missing cert file returns error.
	_, err = tlsx.NewTLSConfig(&tlsx.Config{KeyFile: "key.pem"})
	fmt.Println(err)

	// Missing key file returns error.
	_, err = tlsx.NewTLSConfig(&tlsx.Config{CertFile: "cert.pem"})
	fmt.Println(err)
	// Output:
	// [60701] transport.tls.nil_config: TLS 配置为空
	// [60702] transport.tls.missing_cert: 缺少证书文件
	// [60703] transport.tls.missing_key: 缺少密钥文件
}

func ExampleNewClientTLSConfig_validation() {
	_, err := tlsx.NewClientTLSConfig(nil)
	fmt.Println(err)
	// Output:
	// [60701] transport.tls.nil_config: TLS 配置为空
}
