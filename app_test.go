package actionspin

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

var updateFlag = flag.Bool("update", false, "update fixture files")

func TestApp_Run(t *testing.T) {
	expectedArgs := []struct {
		owner string
		repo  string
		ref   string
	}{
		{"actions", "checkout", "v2"},
		{"actions", "setup-node", "v1"},
		{"actions", "setup-go", "v5"},
		{"actions", "cache", "v4"},
	}
	mockCommitHashResolver := func(ctx context.Context, owner, repo, ref string) (string, error) {
		if !slices.ContainsFunc(expectedArgs, func(e struct{ owner, repo, ref string }) bool {
			return e.owner == owner && e.repo == repo && e.ref == ref
		}) {
			t.Errorf("unexpected call to CommitHashResolver with owner=%s, repo=%s, ref=%s", owner, repo, ref)
		}
		return "mockedCommitHash", nil
	}

	opts := AppOptions{
		Target:             "testdata/.github",
		CommitHashResolver: mockCommitHashResolver,
	}
	if *updateFlag {
		opts.Output = "testdata/fixture"
	} else {
		opts.Output = t.TempDir()
	}

	app := New(opts)
	err := app.Run(context.Background())
	require.NoError(t, err)

	expectedFiles := []string{
		"workflows/test.yaml",
		"workflows/workflow1.yaml",
		"workflows/workflow2.yaml",
		"workflows/workflow3.yaml",
		"workflows/subdir.yaml",
	}
	replacedFiles := app.ReplacedFiles()
	require.ElementsMatch(t, expectedFiles, replacedFiles)

	for _, file := range expectedFiles {
		actual, err := os.ReadFile(filepath.Join(opts.Output, file))
		require.NoError(t, err)
		expected, err := os.ReadFile(filepath.Join("testdata/fixture", file))
		require.NoError(t, err)
		require.Equal(t, string(expected), string(actual))
	}
}
