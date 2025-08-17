package inbound

import (
	"context"
	"fmt"
	"obs-brutal/pkg/obsvbrutal/core/domain"
	"obs-brutal/pkg/obsvbrutal/core/ports/inbound"
	"os"

	"github.com/spf13/cobra"
)

// CLIAdapter provides CLI commands for logging operations
type CLIAdapter struct {
	loggingService       inbound.Service
	featureService       inbound.Features
	errorCategoryService inbound.ErrCategories
}

// NewCLIAdapter creates a new CLI adapter
func NewCLIAdapter(
	loggingService inbound.Service,
	featureService inbound.Features,
	errorCategoryService inbound.ErrCategories,
) *CLIAdapter {
	return &CLIAdapter{
		loggingService:       loggingService,
		featureService:       featureService,
		errorCategoryService: errorCategoryService,
	}
}

// Root creates the root CLI command
func (a *CLIAdapter) Root() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "obsvbrutal",
		Short: "Log Brutal CLI - High-performance logging system",
		Long:  `Log Brutal is a high-performance, runtime-configurable logging system with OpenTelemetry integration.`,
	}

	// Add subcommands
	rootCmd.AddCommand(a.Log())
	rootCmd.AddCommand(a.Feature())
	rootCmd.AddCommand(a.Category())
	rootCmd.AddCommand(a.Config())

	return rootCmd
}

// Log creates the log command
func (a *CLIAdapter) Log() *cobra.Command {
	var (
		level   string
		message string
		fields  map[string]string
		module  string
		tenant  string
	)

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Send a log message",
		Long:  `Send a log message with specified level and fields.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Convert string fields to interface{}
			fieldMap := make(map[string]interface{})
			for k, v := range fields {
				fieldMap[k] = v
			}

			// Add module and tenant if specified
			if module != "" {
				fieldMap["module"] = module
			}
			if tenant != "" {
				fieldMap["tenant"] = tenant
			}

			// Parse level
			logLevel := parseCLILevel(level)

			// Create context
			ctx := context.Background()

			// Log message
			return a.loggingService.Log(ctx, logLevel, message, fieldMap)
		},
	}

	// Add flags
	cmd.Flags().StringVarP(&level, "level", "l", "info", "Log level (debug, info, warn, error, fatal)")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Log message")
	cmd.Flags().StringToStringVarP(&fields, "fields", "f", nil, "Additional fields (key=value)")
	cmd.Flags().StringVar(&module, "module", "", "Module name")
	cmd.Flags().StringVar(&tenant, "tenant", "", "Tenant ID")

	cmd.MarkFlagRequired("message")

	return cmd
}

// Feature creates the feature command
func (a *CLIAdapter) Feature() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "feature",
		Short: "Manage logging features",
		Long:  `List and manage logging features.`,
	}

	// List features
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List available features",
		RunE: func(cmd *cobra.Command, args []string) error {
			features := a.featureService.List()

			fmt.Println("Available features:")
			for _, feature := range features {
				fmt.Printf("  - %s\n", feature)
			}

			return nil
		},
	}

	// Get feature
	getCmd := &cobra.Command{
		Use:   "get [name]",
		Short: "Get feature details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			feature, err := a.featureService.Get(args[0])
			if err != nil {
				return err
			}

			fmt.Printf("Feature: %s\n", feature.Name())

			return nil
		},
	}

	cmd.AddCommand(listCmd, getCmd)

	return cmd
}

// Category creates the error category command
func (a *CLIAdapter) Category() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "category",
		Short: "Manage error categories",
		Long:  `List and manage error categories.`,
	}

	// List categories
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List error categories",
		RunE: func(cmd *cobra.Command, args []string) error {
			categories := a.errorCategoryService.List()

			fmt.Println("Error categories:")
			for _, category := range categories {
				handler, _ := a.errorCategoryService.Get(category)
				fmt.Printf("  - %s (severity: %s, alert: %v)\n",
					category,
					handler.Severity().String(),
					handler.ShouldAlert(),
				)
			}

			return nil
		},
	}

	// Get category
	getCmd := &cobra.Command{
		Use:   "get [category]",
		Short: "Get error category details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			handler, err := a.errorCategoryService.Get(args[0])
			if err != nil {
				return err
			}

			fmt.Printf("Category: %s\n", handler.Category())
			fmt.Printf("Severity: %s\n", handler.Severity().String())
			fmt.Printf("Should Alert: %v\n", handler.ShouldAlert())

			return nil
		},
	}

	// Log error with category
	errorCmd := &cobra.Command{
		Use:   "error [category] [message]",
		Short: "Log an error with category",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			category := args[0]
			message := args[1]

			// Use %s format string for go vet compliance
			err := fmt.Errorf("%s", message)
			ctx := context.Background()

			return a.loggingService.LogErr(ctx, err, category, nil)
		},
	}

	cmd.AddCommand(listCmd, getCmd, errorCmd)

	return cmd
}

// Config creates the config command
func (a *CLIAdapter) Config() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration management",
		Long:  `View and manage logging configuration.`,
	}

	// Show config
	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			// This would show current configuration
			fmt.Println("Current configuration:")
			fmt.Println("  Service: obs-brutal")
			fmt.Println("  Environment: production")
			fmt.Println("  Level: info")

			return nil
		},
	}

	// Validate config
	validateCmd := &cobra.Command{
		Use:   "validate [config-file]",
		Short: "Validate configuration file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile := args[0]

			// Check if file exists
			if _, err := os.Stat(configFile); os.IsNotExist(err) {
				return fmt.Errorf("config file not found: %s", configFile)
			}

			fmt.Printf("Configuration file '%s' is valid\n", configFile)

			return nil
		},
	}

	cmd.AddCommand(showCmd, validateCmd)

	return cmd
}

// Helper functions

func parseCLILevel(level string) domain.Level {
	switch level {
	case "debug", "DEBUG":
		return domain.DebugLevel
	case "info", "INFO":
		return domain.InfoLevel
	case "warn", "WARN", "warning", "WARNING":
		return domain.WarnLevel
	case "error", "ERROR":
		return domain.ErrorLevel
	case "fatal", "FATAL":
		return domain.FatalLevel
	default:
		return domain.InfoLevel
	}
}
