package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// esSearch 搜索操作实现.
type esSearch struct {
	client *elasticsearch.Client
	index  string
}

func (s *esSearch) Query(ctx context.Context, query map[string]any, opts ...SearchOption) (*SearchResult, error) {
	o := applySearchOptions(opts)

	body := map[string]any{
		"query": query,
	}
	if o.from > 0 {
		body["from"] = o.from
	}
	if o.size > 0 {
		body["size"] = o.size
	}
	if len(o.sort) > 0 {
		body["sort"] = o.sort
	}
	if o.highlight != nil {
		body["highlight"] = o.highlight
	}
	applySourceFilter(body, o)

	reader, err := encodeBody(body)
	if err != nil {
		return nil, err
	}

	searchOpts := []func(*esapi.SearchRequest){
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(s.index),
		s.client.Search.WithBody(reader),
	}
	if o.routing != "" {
		searchOpts = append(searchOpts, s.client.Search.WithRouting(o.routing))
	}

	res, err := s.client.Search(searchOpts...)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: search: %w", err)
	}
	defer res.Body.Close()

	return parseSearchResponse(res)
}

func (s *esSearch) Count(ctx context.Context, query map[string]any) (int64, error) {
	body := map[string]any{
		"query": query,
	}

	reader, err := encodeBody(body)
	if err != nil {
		return 0, err
	}

	res, err := s.client.Count(
		s.client.Count.WithContext(ctx),
		s.client.Count.WithIndex(s.index),
		s.client.Count.WithBody(reader),
	)
	if err != nil {
		return 0, fmt.Errorf("elasticsearch: count: %w", err)
	}
	defer res.Body.Close()

	var result struct {
		Count int64 `json:"count"`
	}
	if err := decodeResponse(res, &result); err != nil {
		return 0, err
	}
	return result.Count, nil
}

func (s *esSearch) Aggregate(ctx context.Context, aggs map[string]any, opts ...SearchOption) (*SearchResult, error) {
	o := applySearchOptions(opts)

	body := map[string]any{
		"size": 0,
		"aggs": aggs,
	}
	// 如果有 query 选项可以通过 opts 传入，默认 match_all
	body["query"] = map[string]any{"match_all": map[string]any{}}

	if o.size > 0 {
		body["size"] = o.size
	}
	applySourceFilter(body, o)

	reader, err := encodeBody(body)
	if err != nil {
		return nil, err
	}

	searchOpts := []func(*esapi.SearchRequest){
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(s.index),
		s.client.Search.WithBody(reader),
	}
	if o.routing != "" {
		searchOpts = append(searchOpts, s.client.Search.WithRouting(o.routing))
	}

	res, err := s.client.Search(searchOpts...)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: aggregate: %w", err)
	}
	defer res.Body.Close()

	return parseSearchResponse(res)
}

func (s *esSearch) Scroll(ctx context.Context, query map[string]any, size int, opts ...SearchOption) (*SearchResult, error) {
	o := applySearchOptions(opts)

	scrollDur := o.scrollDuration
	if scrollDur == 0 {
		scrollDur = 1 * time.Minute
	}

	body := map[string]any{
		"query": query,
		"size":  size,
	}
	if len(o.sort) > 0 {
		body["sort"] = o.sort
	}
	applySourceFilter(body, o)

	reader, err := encodeBody(body)
	if err != nil {
		return nil, err
	}

	searchOpts := []func(*esapi.SearchRequest){
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(s.index),
		s.client.Search.WithBody(reader),
		s.client.Search.WithScroll(scrollDur),
	}
	if o.routing != "" {
		searchOpts = append(searchOpts, s.client.Search.WithRouting(o.routing))
	}

	res, err := s.client.Search(searchOpts...)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: scroll: %w", err)
	}
	defer res.Body.Close()

	return parseSearchResponse(res)
}

func (s *esSearch) ClearScroll(ctx context.Context, scrollID string) error {
	res, err := s.client.ClearScroll(
		s.client.ClearScroll.WithContext(ctx),
		s.client.ClearScroll.WithScrollID(scrollID),
	)
	if err != nil {
		return fmt.Errorf("elasticsearch: clear scroll: %w", err)
	}
	defer res.Body.Close()

	return decodeResponse(res, nil)
}

// applySearchOptions 应用搜索选项.
func applySearchOptions(opts []SearchOption) *searchOptions {
	o := &searchOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// applySourceFilter 应用 _source 过滤.
func applySourceFilter(body map[string]any, o *searchOptions) {
	if len(o.sourceIncludes) > 0 || len(o.sourceExcludes) > 0 {
		source := map[string]any{}
		if len(o.sourceIncludes) > 0 {
			source["includes"] = o.sourceIncludes
		}
		if len(o.sourceExcludes) > 0 {
			source["excludes"] = o.sourceExcludes
		}
		body["_source"] = source
	}
}

// parseSearchResponse 解析搜索响应.
func parseSearchResponse(res *esapi.Response) (*SearchResult, error) {
	var raw struct {
		Took     int    `json:"took"`
		ScrollID string `json:"_scroll_id"`
		Hits     struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			MaxScore float64 `json:"max_score"`
			Hits     []struct {
				Index     string              `json:"_index"`
				ID        string              `json:"_id"`
				Score     float64             `json:"_score"`
				Source    json.RawMessage     `json:"_source"`
				Highlight map[string][]string `json:"highlight,omitzero"`
			} `json:"hits"`
		} `json:"hits"`
		Aggregations map[string]json.RawMessage `json:"aggregations,omitzero"`
	}

	if err := decodeResponse(res, &raw); err != nil {
		return nil, err
	}

	result := &SearchResult{
		Took:         raw.Took,
		TotalHits:    raw.Hits.Total.Value,
		MaxScore:     raw.Hits.MaxScore,
		Aggregations: raw.Aggregations,
		ScrollID:     raw.ScrollID,
		Hits:         make([]Hit, 0, len(raw.Hits.Hits)),
	}

	for _, h := range raw.Hits.Hits {
		result.Hits = append(result.Hits, Hit{
			Index:     h.Index,
			ID:        h.ID,
			Score:     h.Score,
			Source:    h.Source,
			Highlight: h.Highlight,
		})
	}

	return result, nil
}
