package debug_test

import (
	"fmt"
	"net/http/httptest"

	"github.com/Tsukikage7/servex/v2/transport/debug"
)

func ExampleNew() {
	h := debug.New(
		debug.WithRoutes([]debug.Route{
			{Method: "GET", Path: "/api/users"},
		}),
		debug.WithBuildVersion("1.0.0"),
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/debug/info", nil)
	h.ServeHTTP(w, r)

	fmt.Println(w.Code)
	// Output:
	// 200
}
