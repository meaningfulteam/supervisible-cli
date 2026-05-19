package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/supervisible/supervisible-cli/internal/api"
	"github.com/supervisible/supervisible-cli/internal/config"
	"github.com/supervisible/supervisible-cli/internal/inputs"
	"github.com/supervisible/supervisible-cli/internal/output"
	"github.com/supervisible/supervisible-cli/internal/schema"
	"github.com/supervisible/supervisible-cli/internal/version"
)

const (
	envAPIKey  = "SUPERVISIBLE_API_KEY"
	envBaseURL = "SUPERVISIBLE_BASE_URL"
)

type contextKey string

const appContextKey contextKey = "app"

// App holds command runtime state.
type App struct {
	store       *config.Store
	cfg         config.Config
	printer     *output.Printer
	baseURL     string
	apiKey      string
	tokenSource string
	timeout     time.Duration
	client      *api.Client
	params      map[string]any
	paramsQuery url.Values
	fields      string
	fieldList   []string
	expand      string
	dryRun      bool
	schema      *schema.Provider
}

func (a *App) Printer() *output.Printer {
	return a.printer
}

func (a *App) BaseURL() string {
	return a.baseURL
}

func (a *App) APIKey() string {
	return a.apiKey
}

func (a *App) TokenSource() string {
	return a.tokenSource
}

func (a *App) ConfigStore() *config.Store {
	return a.store
}

func (a *App) Config() config.Config {
	return a.cfg
}

func (a *App) Fields() string {
	return a.fields
}

func (a *App) DryRun() bool {
	return a.dryRun
}

func (a *App) RequireClient() (*api.Client, error) {
	if strings.TrimSpace(a.apiKey) == "" {
		return nil, fmt.Errorf("missing api key: run 'supervisible auth login' or set %s", envAPIKey)
	}
	if a.client != nil {
		return a.client, nil
	}

	client, err := api.NewClient(a.baseURL, a.apiKey, a.timeout)
	if err != nil {
		return nil, err
	}
	a.client = client
	return client, nil
}

func (a *App) ResolvedQuery(method, endpoint string, base url.Values) url.Values {
	out := cloneQuery(base)

	if a.fields != "" && a.schema != nil && a.schema.SupportsQueryParam(method, endpoint, "fields") {
		out.Set("fields", a.fields)
	}

	if a.expand != "" && a.schema != nil && a.schema.SupportsQueryParam(method, endpoint, "expand") {
		out.Set("expand", a.expand)
	}

	for key, values := range a.paramsQuery {
		for _, value := range values {
			out.Set(key, value)
		}
	}

	return out
}

func (a *App) RequiredScope(method, endpoint string) string {
	if a.schema == nil {
		return ""
	}
	return a.schema.RequiredScope(method, endpoint)
}

func (a *App) PrintData(value any) error {
	projected := value
	if len(a.fieldList) > 0 {
		p, err := output.ProjectFields(value, a.fieldList)
		if err == nil {
			projected = p
		}
	}
	return a.printer.PrintJSON(projected)
}

type RequestPlan struct {
	CommandPath   string     `json:"command_path"`
	Method        string     `json:"method"`
	Endpoint      string     `json:"endpoint"`
	Query         url.Values `json:"query"`
	Body          any        `json:"body,omitempty"`
	RequiredScope string     `json:"required_scope,omitempty"`
	WillExecute   bool       `json:"will_execute"`
}

func (a *App) MaybeDryRun(plan RequestPlan) bool {
	if !a.dryRun {
		return false
	}
	plan.WillExecute = false
	if a.printer.IsJSON() {
		_ = a.printer.PrintJSON(plan)
	} else {
		a.printer.PrintMessage("Dry-run: %s %s", plan.Method, plan.Endpoint)
		if len(plan.Query) > 0 {
			a.printer.PrintMessage("Query: %v", plan.Query)
		}
		if plan.Body != nil {
			data, _ := json.MarshalIndent(plan.Body, "", "  ")
			a.printer.PrintMessage("Body:\n%s", string(data))
		}
		if plan.RequiredScope != "" {
			a.printer.PrintMessage("Required scope: %s", plan.RequiredScope)
		}
	}
	return true
}

func withApp(ctx context.Context, app *App) context.Context {
	return context.WithValue(ctx, appContextKey, app)
}

func appFromCommand(cmd *cobra.Command) (*App, error) {
	value := cmd.Context().Value(appContextKey)
	if value == nil {
		return nil, fmt.Errorf("internal error: runtime context not initialized")
	}
	app, ok := value.(*App)
	if !ok {
		return nil, fmt.Errorf("internal error: invalid runtime context")
	}
	return app, nil
}

// Execute runs the CLI.
func Execute() error {
	return NewRootCommand().Execute()
}

// NewRootCommand creates the supervisible root command.
func NewRootCommand() *cobra.Command {
	var (
		flagJSON       bool
		flagAPIKey     string
		flagBaseURL    string
		flagConfigPath string
		flagTimeout    time.Duration
		flagParams     string
		flagFields     string
		flagExpand     string
		flagDryRun     bool
	)

	root := &cobra.Command{
		Use:               "supervisible",
		Short:             "CLI for Supervisible public API",
		Long:              "supervisible is a CLI for interacting with Supervisible's public API.",
		SilenceUsage:      true,
		SilenceErrors:     true,
		Version:           version.String(),
		PersistentPreRunE: nil,
	}

	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		store, err := config.NewStore(flagConfigPath)
		if err != nil {
			return err
		}
		cfg, err := store.Load()
		if err != nil {
			return err
		}

		resolvedBaseURL, err := resolveBaseURL(flagBaseURL, os.Getenv(envBaseURL), cfg.BaseURL)
		if err != nil {
			return err
		}

		resolvedAPIKey, source, err := resolveAPIKey(store, resolvedBaseURL, flagAPIKey, os.Getenv(envAPIKey))
		if err != nil {
			return err
		}

		paramsObj, err := inputs.ParseJSONObject(flagParams)
		if err != nil {
			return fmt.Errorf("invalid --params: %w", err)
		}
		paramsQuery, err := inputs.ToQueryValues(paramsObj)
		if err != nil {
			return fmt.Errorf("invalid --params values: %w", err)
		}

		schemaProvider, err := schema.NewProvider(cmd.Context())
		if err != nil {
			return fmt.Errorf("load schema: %w", err)
		}

		app := &App{
			store:       store,
			cfg:         cfg,
			printer:     output.NewPrinter(cmd.OutOrStdout(), cmd.ErrOrStderr(), flagJSON),
			baseURL:     resolvedBaseURL,
			apiKey:      resolvedAPIKey,
			tokenSource: source,
			timeout:     flagTimeout,
			params:      paramsObj,
			paramsQuery: paramsQuery,
			fields:      strings.TrimSpace(flagFields),
			fieldList:   output.SplitFieldMask(flagFields),
			expand:      strings.TrimSpace(flagExpand),
			dryRun:      flagDryRun,
			schema:      schemaProvider,
		}

		cmd.SetContext(withApp(cmd.Context(), app))
		return nil
	}

	root.PersistentFlags().BoolVarP(&flagJSON, "json", "j", false, "Output JSON")
	root.PersistentFlags().StringVar(&flagAPIKey, "api-key", "", "API key override (or use SUPERVISIBLE_API_KEY)")
	root.PersistentFlags().StringVar(&flagBaseURL, "base-url", "", "Base URL (host or full /api/v1 URL)")
	root.PersistentFlags().StringVar(&flagConfigPath, "config", "", "Path to config file")
	root.PersistentFlags().DurationVar(&flagTimeout, "timeout", 30*time.Second, "HTTP timeout")
	root.PersistentFlags().StringVar(&flagParams, "params", "", "Raw query params as JSON object")
	root.PersistentFlags().StringVar(&flagFields, "fields", "", "Field mask / projection (comma-separated)")
	root.PersistentFlags().StringVar(&flagExpand, "expand", "", "Expand related objects (comma-separated, e.g. user,project)")
	root.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "Validate and print request plan without executing")

	root.AddCommand(newAuthCommand())
	root.AddCommand(newConfigCommand())
	root.AddCommand(newMeCommand())
	root.AddCommand(newUsersCommand())
	root.AddCommand(newClientsCommand())
	root.AddCommand(newProjectsCommand())
	root.AddCommand(newAssignmentsCommand())
	root.AddCommand(newActualHoursCommand())
	root.AddCommand(newTimeOffCommand())
	root.AddCommand(newSchemaCommand())
	root.AddCommand(newVersionCommand())
	root.AddCommand(newCapacityCommand())
	root.AddCommand(newBenchCommand())
	root.AddCommand(newWhoisCommand())
	root.AddCommand(newContextCommand())

	return root
}

func cloneQuery(query url.Values) url.Values {
	if query == nil {
		return url.Values{}
	}
	out := url.Values{}
	for key, values := range query {
		for _, value := range values {
			out.Add(key, value)
		}
	}
	return out
}

func valueToInt(values url.Values, key string, fallback int) int {
	raw := strings.TrimSpace(values.Get(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func resolveBaseURL(flagValue, envValue, configValue string) (string, error) {
	candidate := strings.TrimSpace(flagValue)
	if candidate == "" {
		candidate = strings.TrimSpace(envValue)
	}
	if candidate == "" {
		candidate = strings.TrimSpace(configValue)
	}
	if candidate == "" {
		candidate = config.DefaultBaseURL
	}
	return api.NormalizeBaseURL(candidate)
}

func resolveAPIKey(store *config.Store, baseURL, flagValue, envValue string) (string, string, error) {
	if v := strings.TrimSpace(flagValue); v != "" {
		return v, "flag", nil
	}
	if v := strings.TrimSpace(envValue); v != "" {
		return v, "env", nil
	}

	token, source, err := store.LoadToken(baseURL)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(token) == "" {
		return "", "", nil
	}
	return token, source, nil
}
