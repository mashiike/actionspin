package actionspin

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
)

type CLI struct {
	LogFormat  string           `help:"Log format" enum:"json,text" default:"json" env:"LOG_FORMAT"`
	Color      bool             `help:"Enable color output" negatable:"" default:"true"`
	LogLevel   string           `help:"Log level" enum:"debug,info,warn,error" default:"info" env:"LOG_LEVEL"`
	Version    kong.VersionFlag `help:"Show version and exit"`
	AppOptions `embed:""`
}

func (c *CLI) Run() int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	k := kong.Parse(c,
		kong.Name("actionspin"),
		kong.Description("Bulk replace GitHub Actions references from version tags to commit hashes for locked, reproducible workflows."),
		kong.UsageOnError(),
		kong.Vars{"version": Version},
	)
	var sLevel slog.Level
	if err := sLevel.UnmarshalText([]byte(c.LogLevel)); err != nil {
		fmt.Fprintf(os.Stderr, "invalid log level: %s\n", c.LogLevel)
		return 1
	}
	logger := newLogger(sLevel, c.LogFormat, c.Color)
	slog.SetDefault(logger)
	if err := c.run(ctx, k); err != nil {
		logger.Error("runtime error", "details", err)
		return 1
	}
	return 0
}

func (c *CLI) run(ctx context.Context, _ *kong.Context) error {
	app := New(c.AppOptions)
	if err := app.Run(ctx); err != nil {
		return err
	}
	uses := app.ReplacedUses()
	if len(uses) == 0 {
		fmt.Println("No replacements found")
		return nil
	}
	var builder strings.Builder
	builder.WriteString("Replaced uses:\n")
	for k, v := range uses {
		builder.WriteString("  - ")
		builder.WriteString(k)
		builder.WriteString(" -> ")
		builder.WriteString(v)
		builder.WriteRune('\n')
	}
	builder.WriteString("\nReplaced files:\n")
	wd, err := os.Getwd()
	if err != nil {
		slog.DebugContext(ctx, "failed to get current working directory", "error", err)
	}
	for _, p := range app.ReplacedFiles() {
		builder.WriteString("  - ")
		absP := filepath.Join(app.opts.Output, p)
		relP, err := filepath.Rel(wd, absP)
		if err != nil {
			relP = absP
		}
		builder.WriteString(relP)
		builder.WriteRune('\n')
	}
	fmt.Println(builder.String())
	return nil
}
