package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	elasticsearch "github.com/elastic/go-elasticsearch/v8"
)

// esDocument 文档操作实现.
type esDocument struct {
	client *elasticsearch.Client
	index  string
}

func (d *esDocument) Index(ctx context.Context, id string, body any) (*IndexResult, error) {
	reader, err := encodeBody(body)
	if err != nil {
		return nil, err
	}

	res, err := d.client.Index(
		d.index,
		reader,
		d.client.Index.WithDocumentID(id),
		d.client.Index.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: index document: %w", err)
	}
	defer res.Body.Close()

	var result IndexResult
	if err := decodeResponse(res, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (d *esDocument) Get(ctx context.Context, id string) (*GetResult, error) {
	res, err := d.client.Get(
		d.index,
		id,
		d.client.Get.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: get document: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, ErrDocumentNotFound
	}

	var result GetResult
	if err := decodeResponse(res, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (d *esDocument) Update(ctx context.Context, id string, body any) (*UpdateResult, error) {
	// ES Update API 要求 body 包裹在 {"doc": ...} 中
	wrapped := map[string]any{"doc": body}
	reader, err := encodeBody(wrapped)
	if err != nil {
		return nil, err
	}

	res, err := d.client.Update(
		d.index,
		id,
		reader,
		d.client.Update.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: update document: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, ErrDocumentNotFound
	}

	var result UpdateResult
	if err := decodeResponse(res, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (d *esDocument) Delete(ctx context.Context, id string) (*DeleteResult, error) {
	res, err := d.client.Delete(
		d.index,
		id,
		d.client.Delete.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: delete document: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 404 {
		return nil, ErrDocumentNotFound
	}

	var result DeleteResult
	if err := decodeResponse(res, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (d *esDocument) Exists(ctx context.Context, id string) (bool, error) {
	res, err := d.client.Exists(
		d.index,
		id,
		d.client.Exists.WithContext(ctx),
	)
	if err != nil {
		return false, fmt.Errorf("elasticsearch: document exists: %w", err)
	}
	defer res.Body.Close()

	return res.StatusCode == 200, nil
}

func (d *esDocument) Bulk(ctx context.Context, actions []BulkAction) (*BulkResult, error) {
	var buf bytes.Buffer

	for _, action := range actions {
		// 操作元数据行
		meta := map[string]any{
			"_id": action.ID,
		}
		if action.Index != "" {
			meta["_index"] = action.Index
		}

		actionLine := map[string]any{
			action.Type: meta,
		}
		if err := json.NewEncoder(&buf).Encode(actionLine); err != nil {
			return nil, fmt.Errorf("elasticsearch: encode bulk action: %w", err)
		}

		// 文档内容行delete 操作不需要
		if action.Type != "delete" && action.Body != nil {
			var bodyToEncode any
			if action.Type == "update" {
				bodyToEncode = map[string]any{"doc": action.Body}
			} else {
				bodyToEncode = action.Body
			}
			if err := json.NewEncoder(&buf).Encode(bodyToEncode); err != nil {
				return nil, fmt.Errorf("elasticsearch: encode bulk body: %w", err)
			}
		}
	}

	res, err := d.client.Bulk(
		&buf,
		d.client.Bulk.WithIndex(d.index),
		d.client.Bulk.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch: bulk: %w", err)
	}
	defer res.Body.Close()

	// 解析 bulk 响应
	var rawResult struct {
		Took   int  `json:"took"`
		Errors bool `json:"errors"`
		Items  []map[string]struct {
			Index  string       `json:"_index"`
			ID     string       `json:"_id"`
			Status int          `json:"status"`
			Error  *ErrorDetail `json:"error,omitempty"`
		} `json:"items"`
	}

	if err := decodeResponse(res, &rawResult); err != nil {
		return nil, err
	}

	result := &BulkResult{
		Took:   rawResult.Took,
		Errors: rawResult.Errors,
		Items:  make([]BulkResultItem, 0, len(rawResult.Items)),
	}

	for _, item := range rawResult.Items {
		for _, v := range item {
			result.Items = append(result.Items, BulkResultItem{
				Index:  v.Index,
				ID:     v.ID,
				Status: v.Status,
				Error:  v.Error,
			})
			break // 每个 item 只有一个操作类型 key
		}
	}

	if result.Errors {
		return result, ErrBulkPartialFailure
	}
	return result, nil
}
