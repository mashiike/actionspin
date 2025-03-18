package actionspin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-github/v70/github"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

type App struct {
	opts          AppOptions
	replacedUses  map[string]string
	replacedFiles map[string]map[string]struct{}
}

type AppOptions struct {
	Target             string                                                             `help:"Replace Target dir or file" required:"" type:"existingpath" default:".github"`
	Output             string                                                             `help:"Output dir" type:"path" default:""`
	GithubToken        string                                                             `help:"GitHub token" env:"GITHUB_TOKEN"`
	CommitHashResolver func(ctx context.Context, owner, repo, ref string) (string, error) `kong:"-"`
}

func New(opts AppOptions) *App {
	if opts.CommitHashResolver == nil {
		opts.CommitHashResolver = DefaultCommitHashResolver(opts.GithubToken)
	}
	if opts.Output == "" {
		opts.Output = opts.Target
	}
	return &App{
		opts:          opts,
		replacedUses:  make(map[string]string),
		replacedFiles: make(map[string]map[string]struct{}),
	}
}

func (app *App) Run(ctx context.Context) error {
	stat, err := os.Stat(app.opts.Target)
	if err != nil {
		return err
	}
	if stat.IsDir() {
		return app.RunDir(ctx, ".")
	}
	return app.RunFile(ctx, app.opts.Target)
}

type SkipableError struct {
	Err error
}

func (e SkipableError) Error() string {
	return e.Err.Error()
}

func (e SkipableError) Unwrap() error {
	return e.Err
}

func Skipable(err error) error {
	return SkipableError{Err: err}
}

func (app *App) RunDir(ctx context.Context, root string) error {
	err := fs.WalkDir(os.DirFS(app.opts.Target), root, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yml" && ext != ".yaml" {
			return nil
		}
		if err := app.RunFile(ctx, path); err != nil {
			var skip SkipableError
			if errors.As(err, &skip) {
				slog.WarnContext(ctx, "failed to run file, skip this file", "path", path, "error", err)
				return nil
			}
			return err
		}
		return nil
	})
	return err
}

var commitHashRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

func (app *App) RunFile(ctx context.Context, path string) error {
	bs, err := os.ReadFile(filepath.Join(app.opts.Target, path))
	if err != nil {
		return Skipable(fmt.Errorf("failed to read file `%s`: %w", path, err))
	}
	var wf Workflow
	if err := yaml.Unmarshal(bs, &wf); err != nil {
		return Skipable(fmt.Errorf("failed to unmarshal file `%s`: %w", path, err))
	}
	if !wf.IsTarget() {
		return Skipable(fmt.Errorf("file `%s` is not target", path))
	}
	for _, job := range wf.Jobs {
		for _, step := range job.Steps {
			if step.Uses == "" {
				continue
			}
			slog.DebugContext(ctx, "detect uses", "path", path, "uses", step.Uses)
			owner, repo, ref, err := parseUses(step.Uses)
			if err != nil {
				slog.WarnContext(ctx, "failed to parse uses, skip this step", "path", path, "uses", step.Uses, "error", err)
				continue
			}
			if commitHashRe.MatchString(ref) {
				slog.DebugContext(ctx, "skip replace uses, ref is commit hash", "path", path, "owner", owner, "repo", repo, "ref", ref)
				continue
			}
			commitHash, err := app.getCommitHash(ctx, owner, repo, ref)
			if err != nil {
				slog.WarnContext(ctx, "failed to resolve commit hash, skip this step", "path", path, "owner", owner, "repo", repo, "ref", ref, "error", err)
				continue
			}
			bs, err = app.replaceUses(ctx, path, bs, owner, repo, ref, commitHash)
			if err != nil {
				return Skipable(fmt.Errorf("failed to replace uses: %w", err))
			}
		}
	}
	outputDir := filepath.Join(app.opts.Output, filepath.Dir(path))
	if _, err := os.Stat(outputDir); err != nil && errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return Skipable(fmt.Errorf("failed to create output dir `%s`: %w", outputDir, err))
		}
	}
	return os.WriteFile(filepath.Join(app.opts.Output, path), bs, 0644)
}

func (app *App) getCommitHash(ctx context.Context, owner, repo, ref string) (string, error) {
	if commitHash, ok := app.replacedUses[owner+"/"+repo+"@"+ref]; ok {
		return commitHash, nil
	}
	commitHash, err := app.opts.CommitHashResolver(ctx, owner, repo, ref)
	if err != nil {
		return "", err
	}
	app.replacedUses[owner+"/"+repo+"@"+ref] = commitHash
	return commitHash, nil
}

func (app *App) replaceUses(ctx context.Context, path string, bs []byte, owner, repo, ref string, commitHash string) ([]byte, error) {
	uses := owner + "/" + repo + "@" + ref
	if replacedUsesInFile, ok := app.replacedFiles[path]; ok {
		if _, ok := replacedUsesInFile[uses]; ok {
			return bs, nil
		}
	} else {
		app.replacedFiles[path] = make(map[string]struct{})
	}
	replaceStr := fmt.Sprintf("%s/%s@%s # %s", owner, repo, commitHash, ref)
	slog.DebugContext(ctx, "replace uses for debug", "path", path, "before", uses, "after", replaceStr)
	slog.InfoContext(ctx, "replace uses", "path", path, "owner", owner, "repo", repo, "ref", ref, "commitHash", commitHash)
	bs = bytes.ReplaceAll(bs, []byte(uses), []byte(replaceStr))
	app.replacedFiles[path][uses] = struct{}{}
	app.replacedUses[uses] = commitHash
	return bs, nil
}

func parseUses(uses string) (string, string, string, error) {
	parts := strings.SplitN(uses, "@", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("unexpected uses format, expected `owner/repo@ref`, but got `%s`", uses)
	}
	ownerRepo := parts[0]
	ref := parts[1]
	parts = strings.SplitN(ownerRepo, "/", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("unexpected owner/repo format, expected `owner/repo`, but got `%s`", ownerRepo)
	}
	owner := parts[0]
	repo := parts[1]
	return owner, repo, ref, nil
}

func (app *App) ReplacedUses() map[string]string {
	return app.replacedUses
}

func (app *App) ReplacedFiles() []string {
	var replacedFiles []string
	for path := range app.replacedFiles {
		replacedFiles = append(replacedFiles, path)
	}
	return replacedFiles
}

type Workflow struct {
	Name string         `yaml:"name"`
	On   any            `yaml:"on"`
	Jobs map[string]Job `yaml:"jobs"`
}

func (wf *Workflow) IsTarget() bool {
	return wf.Name != "" && wf.On != nil && len(wf.Jobs) > 0
}

type Job struct {
	Name  string `yaml:"name"`
	Steps []Step `yaml:"steps"`
}

type Step struct {
	Name string `yaml:"name"`
	Uses string `yaml:"uses"`
}

func DefaultCommitHashResolver(token string) func(ctx context.Context, owner, repo, ref string) (string, error) {
	var client *github.Client
	if token != "" {
		client = github.NewClient(oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})))
	} else {
		client = github.NewClient(nil)
	}
	return func(ctx context.Context, owner, repo, ref string) (string, error) {
		// First, try resolving as a branch reference.
		branchRef, _, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+ref)
		if err == nil && branchRef.Object != nil {
			return branchRef.Object.GetSHA(), nil
		}

		tagRef, resp, err := client.Git.GetRef(ctx, owner, repo, "refs/tags/"+ref)
		if err == nil && tagRef.Object != nil {
			if tagRef.Object.GetType() == "tag" {
				tagObj, _, err := client.Git.GetTag(ctx, owner, repo, tagRef.Object.GetSHA())
				if err != nil {
					return "", fmt.Errorf("failed to resolve annotated tag: %w", err)
				}
				return tagObj.Object.GetSHA(), nil
			}
			return tagRef.Object.GetSHA(), nil
		}

		return "", fmt.Errorf("failed to resolve ref, status code `%d`: %w", resp.StatusCode, err)
	}
}
