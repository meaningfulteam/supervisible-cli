package schema

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed openapi/public-api-v1.json
var embeddedOpenAPI []byte

type OpenAPI struct {
	Info struct {
		Title       string `json:"title"`
		Version     string `json:"version"`
		Description string `json:"description"`
	} `json:"info"`
	Paths map[string]map[string]Operation `json:"paths"`
}

type Operation struct {
	Summary       string                    `json:"summary"`
	Description   string                    `json:"description"`
	OperationID   string                    `json:"operationId"`
	Tags          []string                  `json:"tags"`
	RequiredScope string                    `json:"x-required-scope"`
	Parameters    []Parameter               `json:"parameters"`
	RequestBody   map[string]any            `json:"requestBody"`
	Responses     map[string]map[string]any `json:"responses"`
}

type Parameter struct {
	In          string         `json:"in"`
	Name        string         `json:"name"`
	Required    bool           `json:"required"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}

type Endpoint struct {
	Operation     string `json:"operation"`
	Method        string `json:"method"`
	Path          string `json:"path"`
	Summary       string `json:"summary"`
	RequiredScope string `json:"required_scope"`
}

type Provider struct {
	spec OpenAPI
}

func NewProvider(ctx context.Context) (*Provider, error) {
	data, err := resolveSchemaData(ctx)
	if err != nil {
		return nil, err
	}

	var spec OpenAPI
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse openapi schema: %w", err)
	}

	if len(spec.Paths) == 0 {
		return nil, fmt.Errorf("openapi schema has no paths")
	}

	return &Provider{spec: spec}, nil
}

func (p *Provider) SupportsQueryParam(method, path, name string) bool {
	op, ok := p.Lookup(method, path)
	if !ok {
		return false
	}
	for _, param := range op.Parameters {
		if strings.EqualFold(param.In, "query") && strings.EqualFold(param.Name, name) {
			return true
		}
	}
	return false
}

func (p *Provider) RequiredScope(method, path string) string {
	op, ok := p.Lookup(method, path)
	if !ok {
		return ""
	}
	return op.RequiredScope
}

func (p *Provider) Lookup(method, path string) (Operation, bool) {
	methods, ok := p.spec.Paths[path]
	if !ok {
		return Operation{}, false
	}
	op, ok := methods[strings.ToLower(method)]
	if !ok {
		return Operation{}, false
	}
	return op, true
}

func (p *Provider) Endpoints() []Endpoint {
	out := make([]Endpoint, 0)
	for path, methods := range p.spec.Paths {
		for method, op := range methods {
			operation := normalizeOperation(method, path)
			if op.OperationID != "" {
				operation = op.OperationID
			}
			out = append(out, Endpoint{
				Operation:     operation,
				Method:        strings.ToUpper(method),
				Path:          path,
				Summary:       op.Summary,
				RequiredScope: op.RequiredScope,
			})
		}
	}
	return out
}

func (p *Provider) Describe(selector string) (Endpoint, Operation, bool) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return Endpoint{}, Operation{}, false
	}

	if strings.Contains(selector, " ") {
		parts := strings.SplitN(selector, " ", 2)
		method := strings.TrimSpace(parts[0])
		path := strings.TrimSpace(parts[1])
		op, ok := p.Lookup(method, path)
		if !ok {
			return Endpoint{}, Operation{}, false
		}
		return Endpoint{
			Operation:     normalizeOperation(method, path),
			Method:        strings.ToUpper(method),
			Path:          path,
			Summary:       op.Summary,
			RequiredScope: op.RequiredScope,
		}, op, true
	}

	for path, methods := range p.spec.Paths {
		for method, op := range methods {
			normalized := normalizeOperation(method, path)
			if strings.EqualFold(selector, normalized) || strings.EqualFold(selector, op.OperationID) {
				return Endpoint{
					Operation:     normalized,
					Method:        strings.ToUpper(method),
					Path:          path,
					Summary:       op.Summary,
					RequiredScope: op.RequiredScope,
				}, op, true
			}
		}
	}

	return Endpoint{}, Operation{}, false
}

func resolveSchemaData(ctx context.Context) ([]byte, error) {
	if rawURL := strings.TrimSpace(os.Getenv("SUPERVISIBLE_SCHEMA_URL")); rawURL != "" {
		data, err := fetchURL(ctx, rawURL)
		if err != nil {
			return nil, fmt.Errorf("load schema from SUPERVISIBLE_SCHEMA_URL: %w", err)
		}
		return data, nil
	}

	if filePath := strings.TrimSpace(os.Getenv("SUPERVISIBLE_SCHEMA_FILE")); filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("load schema from SUPERVISIBLE_SCHEMA_FILE: %w", err)
		}
		return data, nil
	}

	return embeddedOpenAPI, nil
}

func fetchURL(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func normalizeOperation(method, apiPath string) string {
	method = strings.ToLower(method)
	segments := strings.Split(strings.Trim(apiPath, "/"), "/")
	if len(segments) == 0 {
		return method
	}

	parts := make([]string, 0, len(segments))
	for _, seg := range segments {
		if seg == "" || strings.HasPrefix(seg, "{") {
			continue
		}
		parts = append(parts, strings.ReplaceAll(seg, "-", "_"))
	}
	if len(parts) == 0 {
		return method
	}
	return strings.Join(parts, ".") + "." + method
}
