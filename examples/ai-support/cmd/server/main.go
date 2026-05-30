package main

import (
	"log"
	"net/http"
	"os"

	supportagent "github.com/Tsukikage7/servex/v2/examples/ai-support/internal/agent"
	supporthttp "github.com/Tsukikage7/servex/v2/examples/ai-support/internal/http"
	llmclient "github.com/Tsukikage7/servex/v2/examples/ai-support/internal/llm"
	"github.com/Tsukikage7/servex/v2/examples/ai-support/internal/tools"
)

func main() {
	model := llmclient.NewFromEnv()
	agent := supportagent.New(model, tools.NewRegistry())
	handler := supporthttp.NewChatHandler(agent)

	mux := http.NewServeMux()
	mux.Handle("POST /chat", handler)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	addr := getenv("ADDR", ":8080")
	log.Printf("ai-support listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
