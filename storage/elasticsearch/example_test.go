package elasticsearch_test

import (
	"fmt"

	"github.com/Tsukikage7/servex/storage/elasticsearch"
)

func ExampleDefaultConfig() {
	cfg := elasticsearch.DefaultConfig()
	fmt.Println("addresses:", cfg.Addresses)
	fmt.Println("max retries:", cfg.MaxRetries)
	// Output:
	// addresses: [http://localhost:9200]
	// max retries: 3
}

func ExampleConfig_Validate() {
	cfg := &elasticsearch.Config{Addresses: []string{"http://localhost:9200"}}
	fmt.Println("valid:", cfg.Validate() == nil)

	// No addresses or cloud ID.
	bad := &elasticsearch.Config{}
	fmt.Println("empty:", bad.Validate())
	// Output:
	// valid: true
	// empty: elasticsearch: addresses is empty
}

func ExampleConfig_ApplyDefaults() {
	cfg := &elasticsearch.Config{
		Addresses: []string{"http://es:9200"},
	}
	cfg.ApplyDefaults()

	fmt.Println("max retries:", cfg.MaxRetries)
	fmt.Println("max idle conns:", cfg.MaxIdleConnsPerHost)
	// Output:
	// max retries: 3
	// max idle conns: 10
}
