package tlsx_test

import (
	"fmt"

	tlsx "github.com/Tsukikage7/servex/v2/transport/tls"
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
	// tls: config is nil
	// tls: cert_file is required
	// tls: key_file is required
}

func ExampleNewClientTLSConfig_validation() {
	_, err := tlsx.NewClientTLSConfig(nil)
	fmt.Println(err)
	// Output:
	// tls: config is nil
}
