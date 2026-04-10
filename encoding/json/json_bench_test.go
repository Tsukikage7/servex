package json

import (
	"testing"
)

// --- test structs ---

type smallStruct struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type mediumStruct struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	Age     int      `json:"age"`
	Active  bool     `json:"active"`
	Tags    []string `json:"tags"`
	Address struct {
		Street string `json:"street"`
		City   string `json:"city"`
		Zip    string `json:"zip"`
	} `json:"address"`
}

type largeStruct struct {
	ID          int             `json:"id"`
	Name        string          `json:"name"`
	Email       string          `json:"email"`
	Age         int             `json:"age"`
	Active      bool            `json:"active"`
	Tags        []string        `json:"tags"`
	Scores      []float64       `json:"scores"`
	Metadata    map[string]any  `json:"metadata"`
	Items       []mediumStruct  `json:"items"`
	Preferences map[string]bool `json:"preferences"`
}

var (
	smallVal = smallStruct{ID: 1, Name: "alice"}

	mediumVal = mediumStruct{
		ID: 42, Name: "bob", Email: "bob@example.com", Age: 30, Active: true,
		Tags: []string{"admin", "user", "editor"},
		Address: struct {
			Street string `json:"street"`
			City   string `json:"city"`
			Zip    string `json:"zip"`
		}{Street: "123 Main St", City: "Metropolis", Zip: "12345"},
	}

	largeVal = func() largeStruct {
		items := make([]mediumStruct, 10)
		for i := range items {
			items[i] = mediumVal
			items[i].ID = i
		}
		return largeStruct{
			ID: 1, Name: "charlie", Email: "charlie@example.com", Age: 25, Active: true,
			Tags:   []string{"a", "b", "c", "d", "e"},
			Scores: []float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8, 9.9},
			Metadata: map[string]any{
				"key1": "value1", "key2": 42, "key3": true,
				"nested": map[string]any{"a": 1, "b": "two"},
			},
			Items:       items,
			Preferences: map[string]bool{"dark_mode": true, "notifications": false, "analytics": true},
		}
	}()

	c = codec{}
)

// --- Marshal ---

func BenchmarkMarshal_Small(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.Marshal(smallVal)
	}
}

func BenchmarkMarshal_Medium(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.Marshal(mediumVal)
	}
}

func BenchmarkMarshal_Large(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		c.Marshal(largeVal)
	}
}

// --- Unmarshal ---

func BenchmarkUnmarshal_Small(b *testing.B) {
	data, _ := c.Marshal(smallVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var v smallStruct
		c.Unmarshal(data, &v)
	}
}

func BenchmarkUnmarshal_Medium(b *testing.B) {
	data, _ := c.Marshal(mediumVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var v mediumStruct
		c.Unmarshal(data, &v)
	}
}

func BenchmarkUnmarshal_Large(b *testing.B) {
	data, _ := c.Marshal(largeVal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		var v largeStruct
		c.Unmarshal(data, &v)
	}
}
