//go:build integration
// +build integration

package exec

import (
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/B-S-F/onyx/pkg/helper"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

var (
	update = flag.Bool("update", false, "update the golden files of this test")
)

func TestExecCommandIntegration(t *testing.T) {
	t.Run("test exec integration", func(t *testing.T) {
		var gotObj, wantObj map[string]interface{}
		tempDir := t.TempDir()
		resultFile := filepath.Join(tempDir, "qg-result.yaml")
		evidenceZipFile := filepath.Join(tempDir, "evidence.zip")
		serveApps("testdata/apps", "8081", t)

		cmd := ExecCommand()
		cmd.SetArgs([]string{
			"testdata/configuration",
			"--output-dir", tempDir,
			"--check-timeout", "3",
		})
		startTime := time.Now()
		err := cmd.Execute()
		endTime := time.Now()
		diff := endTime.Sub(startTime)
		assert.NoError(t, err)
		assert.Less(t, diff.Seconds(), 10.0)

		assert.FileExists(t, resultFile)
		assert.FileExists(t, evidenceZipFile)
		got, err := os.ReadFile(resultFile)
		if err != nil {
			t.Fatal(err)
		}
		want := helper.GoldenValue(t, "configuration/qg-result.golden", got, *update)
		err = yaml.Unmarshal(got, &gotObj)
		if err != nil {
			t.Fatal(err)
		}
		err = yaml.Unmarshal(want, &wantObj)
		if err != nil {
			t.Fatal(err)
		}

		// ignoring the date, as it changes on each run
		ignoreKeys := []string{"date", "evidencePath"}

		// ignoring the bash line number, as it differs between different Bash versions
		ignorePattern := "/bin/bash: line [0-9]+:"

		if !helper.MapsEqual(gotObj, wantObj, ignoreKeys, ignorePattern) {
			t.Fail()
			assert.Equal(t, string(want), string(got))
			t.Logf("ignoreKeys: %v", ignoreKeys)
			t.Logf("golden file '%s' does not match the actual result", "qg-result.golden")
		}
	})
}

func serveApps(directory string, port string, t *testing.T) {
	http.Handle("/", http.FileServer(http.Dir(directory)))
	go func() {
		err := http.ListenAndServe(":"+port, nil)
		if err != nil {
			t.Fail()
		}
	}()
}
