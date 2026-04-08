package httpserver_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/Tsukikage7/servex/observability/logger"
	"github.com/Tsukikage7/servex/transport/httpserver"
)

func ExampleNew() {
	log := logger.MustNewLogger(&logger.Config{Level: "info", Format: "console", Output: "console"})

	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello")
	})

	srv := httpserver.New(mux,
		httpserver.WithLogger(log),
		httpserver.WithAddr(":8080"),
		httpserver.WithRecovery(),
	)

	fmt.Println(srv.Name())
	fmt.Println(srv.Addr())
	// Output:
	// HTTP
	// :8080
}

func ExampleNewRouter() {
	log := logger.MustNewLogger(&logger.Config{Level: "info", Format: "console", Output: "console"})

	router := httpserver.NewRouter()
	router.GET("/ping", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "pong")
	}))

	srv := httpserver.New(router,
		httpserver.WithLogger(log),
		httpserver.WithAddr(":9090"),
	)

	// 使用 httptest 验证路由.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ping", nil)
	srv.Handler().ServeHTTP(w, r)

	fmt.Println(srv.Addr())
	fmt.Println(w.Body.String())
	// Output:
	// :9090
	// pong
}
