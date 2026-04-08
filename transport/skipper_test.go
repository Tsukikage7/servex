package transport

import "testing"

func TestBuildMethodSkipper(t *testing.T) {
	t.Run("精确匹配", func(t *testing.T) {
		skip := BuildMethodSkipper([]string{
			"/api.user.v1.AuthService/Login",
			"/api.user.v1.AuthService/Register",
		})

		if !skip("/api.user.v1.AuthService/Login") {
			t.Error("should skip Login")
		}
		if !skip("/api.user.v1.AuthService/Register") {
			t.Error("should skip Register")
		}
		if skip("/api.user.v1.AuthService/GetProfile") {
			t.Error("should not skip GetProfile")
		}
	})

	t.Run("前缀通配", func(t *testing.T) {
		skip := BuildMethodSkipper([]string{
			"/api.user.v1.AuthService/*",
		})

		if !skip("/api.user.v1.AuthService/Login") {
			t.Error("should skip Login under AuthService")
		}
		if !skip("/api.user.v1.AuthService/Register") {
			t.Error("should skip Register under AuthService")
		}
		if skip("/api.product.v1.ProductService/List") {
			t.Error("should not skip ProductService methods")
		}
	})

	t.Run("混合匹配", func(t *testing.T) {
		skip := BuildMethodSkipper([]string{
			"/api.user.v1.AuthService/*",
			"/api.product.v1.ProductService/List",
		})

		if !skip("/api.user.v1.AuthService/Login") {
			t.Error("should skip AuthService methods")
		}
		if !skip("/api.product.v1.ProductService/List") {
			t.Error("should skip ProductService/List")
		}
		if skip("/api.product.v1.ProductService/Create") {
			t.Error("should not skip ProductService/Create")
		}
	})

	t.Run("空列表不匹配任何方法", func(t *testing.T) {
		skip := BuildMethodSkipper(nil)

		if skip("/api.user.v1.AuthService/Login") {
			t.Error("empty skipper should not skip anything")
		}
	})

	t.Run("空方法名不匹配", func(t *testing.T) {
		skip := BuildMethodSkipper([]string{
			"/api.user.v1.AuthService/Login",
		})

		if skip("") {
			t.Error("empty method should not match")
		}
	})
}
