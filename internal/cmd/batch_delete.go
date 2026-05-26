package cmd

import (
	"context"
	"fmt"
)

// batchDeleteResult is the per-ID outcome shape used by JSON output.
type batchDeleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
	Error   string `json:"error,omitempty"`
}

// runBatchDelete iterates ids, calling delete for each. With continueOnError
// false, it stops at the first failure and returns that error wrapped. With
// it true, it tries every ID and returns nil unless ALL failed.
//
// Per-ID success emits "<entity> deleted: <id>" on stderr; the JSON path is
// always a flat array of {id, deleted, error} so consumers can pipe through
// jq regardless of batch size.
func runBatchDelete(
	ctx context.Context,
	app *App,
	entity string,
	ids []string,
	deleteOne func(ctx context.Context, id string) error,
	continueOnError bool,
) error {
	results := make([]batchDeleteResult, 0, len(ids))
	successes := 0
	var firstErr error

	for _, id := range ids {
		err := deleteOne(ctx, id)
		if err != nil {
			results = append(results, batchDeleteResult{ID: id, Deleted: false, Error: err.Error()})
			if firstErr == nil {
				firstErr = fmt.Errorf("delete %s: %w", id, err)
			}
			if !continueOnError {
				break
			}
			continue
		}
		results = append(results, batchDeleteResult{ID: id, Deleted: true})
		successes++
		app.Printer().Aux("Deleted %s: %s", entity, id)
	}

	if app.Printer().IsJSON() {
		if err := app.PrintData(results); err != nil {
			return err
		}
	}
	if len(ids) > 1 {
		app.Printer().Aux("batch delete: %d/%d succeeded", successes, len(ids))
	}
	if continueOnError && successes > 0 {
		// At least one delete worked — surface success even if others failed.
		// The JSON output already carries the per-ID errors for the caller.
		return nil
	}
	return firstErr
}

// previewBatchDelete prints the dry-run plan for each ID. JSON mode returns
// the full list of RequestPlans on stdout; non-JSON prints one "Dry-run: ..."
// line per ID to stderr.
func previewBatchDelete(app *App, commandPath, endpointPattern, pathPrefix, requiredScope string, ids []string) error {
	if app.Printer().IsJSON() {
		plans := make([]RequestPlan, 0, len(ids))
		for _, id := range ids {
			plans = append(plans, RequestPlan{
				CommandPath:   commandPath,
				Method:        "DELETE",
				Endpoint:      pathPrefix + id,
				RequiredScope: requiredScope,
			})
		}
		return app.PrintData(plans)
	}
	for _, id := range ids {
		app.Printer().Aux("Dry-run: DELETE %s%s", pathPrefix, id)
	}
	if requiredScope != "" {
		app.Printer().Aux("Required scope: %s", requiredScope)
	}
	_ = endpointPattern // reserved for future per-endpoint hints
	return nil
}
