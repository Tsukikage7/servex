package apikey_test

import (
	"context"
	"fmt"

	"github.com/Tsukikage7/servex/v2/auth"
	"github.com/Tsukikage7/servex/v2/auth/apikey"
)

func ExampleAuthenticator() {
	// 创建静态 API Key 验证器.
	keys := map[string]*auth.Principal{
		"sk-test-key-001": {
			ID:   "svc-payment",
			Type: auth.PrincipalTypeService,
			Name: "Payment Service",
		},
	}
	authenticator := apikey.New(apikey.StaticValidator(keys))

	// 验证有效的 API Key.
	principal, err := authenticator.Authenticate(context.Background(), auth.Credentials{
		Type:  auth.CredentialTypeAPIKey,
		Token: "sk-test-key-001",
	})
	fmt.Println(err)
	fmt.Println(principal.ID)
	fmt.Println(principal.Name)

	// 验证无效的 API Key.
	_, err = authenticator.Authenticate(context.Background(), auth.Credentials{
		Type:  auth.CredentialTypeAPIKey,
		Token: "invalid-key",
	})
	fmt.Println(err)
	// Output:
	// <nil>
	// svc-payment
	// Payment Service
	// auth: 无效凭据
}
