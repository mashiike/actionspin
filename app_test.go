package actionspin

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var updateFlag = flag.Bool("update", false, "update fixture files")

func TestApp_Run(t *testing.T) {
	mockCommitHashResolver := func(ctx context.Context, owner, repo, ref string) (string, error) {
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
