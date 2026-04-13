package elasticsearch

import (
	"context"
	"fmt"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

// esIndex 索引操作实现.
type esIndex struct {
	client *es.Client
	name   string
	log    logger.Logger
}

func (idx *esIndex) Create(ctx context.Context, body map[string]any) error {
	var opts []func(*esapi.IndicesCreateRequest)
	opts = append(opts, idx.client.Indices.Create.WithContext(ctx))

	if body != nil {
		reader, err := encodeBody(body)
		if err != nil {
			return err
		}
		opts = append(opts, idx.client.Indices.Create.WithBody(reader))
	}

	res, err := idx.client.Indices.Create(idx.name, opts...)
	if err != nil {
		return fmt.Errorf("elasticsearch: create index: %w", err)
	}
	defer res.Body.Close()

	return decodeResponse(res, nil)
}

func (idx *esIndex) Delete(ctx context.Context) error {
	res, err := idx.client.Indices.Delete(
		[]string{idx.name},
		idx.client.Indices.Delete.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("elasticsearch: delete index: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return ErrIndexNotFound
	}
	return decodeResponse(res, nil)
}

func (idx *esIndex) Exists(ctx context.Context) (bool, error) {
	res, err := idx.client.Indices.Exists(
		[]string{idx.name},
		idx.client.Indices.Exists.WithContext(ctx),
	)
	if err != nil {
		return false, fmt.Errorf("elasticsearch: index exists: %w", err)
	}
	defer res.Body.Close()

	return res.StatusCode == 200, nil
}

func (idx *esIndex) PutMapping(ctx context.Context, body map[string]any) error {
	reader, err := encodeBody(body)
	if err != nil {
		return err
	}

	res, err := idx.client.Indices.PutMapping(
		[]string{idx.name},
		reader,
		idx.client.Indices.PutMapping.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("elasticsearch: put mapping: %w", err)
	}
	defer res.Body.Close()

	return decodeResponse(res, nil)
}

func (idx *esIndex) GetMapping(ctx context.Context) (map[string]any, error) {
	res, err := idx.client.Indices.GetMapping(
		idx.client.Indices.GetMapping.WithIndex(idx.name),
		idx.client.Indices.GetMapping.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: get mapping: %w", err)
	}
	defer res.Body.Close()

	var result map[string]any
	if err := decodeResponse(res, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (idx *esIndex) PutSettings(ctx context.Context, body map[string]any) error {
	reader, err := encodeBody(body)
	if err != nil {
		return err
	}

	res, err := idx.client.Indices.PutSettings(
		reader,
		idx.client.Indices.PutSettings.WithIndex(idx.name),
		idx.client.Indices.PutSettings.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("elasticsearch: put settings: %w", err)
	}
	defer res.Body.Close()

	return decodeResponse(res, nil)
}

func (idx *esIndex) PutAlias(ctx context.Context, alias string) error {
	res, err := idx.client.Indices.PutAlias(
		[]string{idx.name},
		alias,
		idx.client.Indices.PutAlias.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("elasticsearch: put alias: %w", err)
	}
	defer res.Body.Close()

	return decodeResponse(res, nil)
}

func (idx *esIndex) DeleteAlias(ctx context.Context, alias string) error {
	res, err := idx.client.Indices.DeleteAlias(
		[]string{idx.name},
		[]string{alias},
		idx.client.Indices.DeleteAlias.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("elasticsearch: delete alias: %w", err)
	}
	defer res.Body.Close()

	return decodeResponse(res, nil)
}

func (idx *esIndex) Document() Document {
	return &esDocument{
		client: idx.client,
		index:  idx.name,
	}
}

func (idx *esIndex) Search() Search {
	return &esSearch{
		client: idx.client,
		index:  idx.name,
	}
}
