package notify

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

// ElasticsearchBackend publishes notifications to an Elasticsearch index.
type ElasticsearchBackend struct {
	url    string
	index  string
	client *http.Client
}

// NewElasticsearchBackend creates an Elasticsearch notification backend.
func NewElasticsearchBackend(url, index string) *ElasticsearchBackend {
	if index == "" {
		index = "s3-events"
	}
	return &ElasticsearchBackend{
		url:    url,
		index:  index,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (e *ElasticsearchBackend) Name() string {
	return "elasticsearch"
}

// Publish indexes an event document into Elasticsearch.
func (e *ElasticsearchBackend) Publish(ctx context.Context, payload []byte) error {
	docURL := fmt.Sprintf("%s/%s/_doc", e.url, e.index)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, docURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("elasticsearch request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("elasticsearch publish: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("elasticsearch returned %d", resp.StatusCode)
	}
	return nil
}

func (e *ElasticsearchBackend) Close() error {
	return nil
}
