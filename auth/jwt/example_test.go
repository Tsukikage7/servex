package jwt_test

import (
	"context"
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"

	"github.com/Tsukikage7/servex/auth/jwt"
	"github.com/Tsukikage7/servex/observability/logger"
)

func ExampleNewJWT() {
	log := logger.MustNewLogger(&logger.Config{Level: "info", Format: "console", Output: "console"})

	j := jwt.NewJWT(
		jwt.WithSecretKey("my-secret-key-at-least-32-bytes!"),
		jwt.WithIssuer("my-service"),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	fmt.Println(j.Issuer())
	fmt.Println(j.Name())
	// Output:
	// my-service
	// JWT
}

func ExampleJWT_Generate() {
	log := logger.MustNewLogger(&logger.Config{Level: "info", Format: "console", Output: "console"})

	j := jwt.NewJWT(
		jwt.WithSecretKey("my-secret-key-at-least-32-bytes!"),
		jwt.WithIssuer("my-service"),
		jwt.WithLogger(log),
		jwt.WithTokenPrefix(""),
	)

	// 生成令牌.
	claims := &jwt.StandardClaims{
		RegisteredClaims: gojwt.RegisteredClaims{
			Subject:   "user-123",
			Issuer:    "my-service",
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
		},
	}

	token, err := j.Generate(context.Background(), claims)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("token generated:", token != "")

	// 验证令牌.
	parsed, err := j.Validate(context.Background(), token)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	subject, _ := parsed.GetSubject()
	fmt.Println("subject:", subject)
	// Output:
	// token generated: true
	// subject: user-123
}
