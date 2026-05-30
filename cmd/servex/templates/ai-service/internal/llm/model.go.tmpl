package llm

import (
	"os"
	"time"

	servexllm "github.com/Tsukikage7/servex/v2/llm"
	"github.com/Tsukikage7/servex/v2/llm/middleware"
	"github.com/Tsukikage7/servex/v2/llm/provider/openai"
)

func NewFromEnv() servexllm.ChatModel {
	apiKey := getenv("OPENAI_API_KEY", "sk-local")
	baseURL := getenv("OPENAI_BASE_URL", "https://api.openai.com/v1")
	model := getenv("OPENAI_MODEL", "gpt-4o-mini")

	client := openai.New(apiKey,
		openai.WithBaseURL(baseURL),
		openai.WithModel(model),
	)

	return middleware.Chain(
		middleware.Retry(3, 300*time.Millisecond),
	)(client)
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
