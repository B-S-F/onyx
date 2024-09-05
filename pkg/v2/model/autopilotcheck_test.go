package model

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/B-S-F/onyx/pkg/configuration"
	"github.com/B-S-F/onyx/pkg/logger"
	"github.com/B-S-F/onyx/pkg/runner"
	"github.com/B-S-F/onyx/pkg/v2/executor"
	"github.com/B-S-F/onyx/pkg/workdir"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestAutopilotExecuteIntegration(t *testing.T) {
	item :=
		Item{
			Chapter: configuration.Chapter{
				Id: "chapter",
			},
			Requirement: configuration.Requirement{
				Id: "requirement",
			},
			Check: configuration.Check{
				Id: "check",
			},
		}
	testCases := map[string]struct {
		check  *AutopilotCheck
		strict bool
		want   func(tmpDir string) *AutopilotResult
	}{
		"should return correct result": {
			strict: false,
			check: &AutopilotCheck{
				Item: item,
				Autopilot: Autopilot{
					Name: "autopilot",
					Steps: [][]Step{{
						Step{
							Title: "write hello word",
							Configs: map[string]string{
								"config1": "value1",
								"config2": "value2",
							},
							Depends: []string{},
							ID:      "write",
							Run:     "echo '{\"key\": \"hello world\"}' > $AUTOPILOT_OUTPUT_DIR/data.txt\necho 'done writing'",
						},
						Step{
							Title: "say hello word",
							Configs: map[string]string{
								"config1": "value1",
								"config2": "value2",
							},
							Depends: []string{"write"},
							ID:      "echo",
							Run:     "cat $AUTOPILOT_INPUT_DIRS/data.txt > $AUTOPILOT_RESULT_FILE\necho 'done echoing'",
						},
					}},
					Env: map[string]string{
						"ENV_VAR1": "value1",
						"ENV_VAR2": "value2",
					},
					Evaluate: Evaluate{
						Run: "data=$(cat \"$EVALUATOR_INPUT_FILES\"); expected='{\"key\": \"hello world\"}'; [[ \"$data\" == \"$expected\" ]] && echo '{\"status\": \"GREEN\", \"reason\": \"file matches\", \"result\": {\"criterion\": \"criteria1\", \"fulfilled\": true, \"justification\": \"reason1\", \"metadata\": {\"severity\": \"HIGH\", \"package\": \"package1\"}}}' || echo '{\"status\": \"RED\", \"reason\": \"file does not matches\", \"result\": {\"criterion\": \"criteria1\", \"fulfilled\": false, \"justification\": \"reason1\", \"metadata\": {\"severity\": \"HIGH\", \"package\": \"package1\"}}}'",
					},
				},
			},
			want: func(tmpDir string) *AutopilotResult {
				return &AutopilotResult{
					StepResults: []StepResult{
						{Logs: []string{"done writing"}, ID: "write", OutputDir: fmt.Sprintf("%s/chapter_requirement_check/steps/write/files", tmpDir)},
						{Logs: []string{"done echoing"}, ID: "echo", OutputDir: fmt.Sprintf("%s/chapter_requirement_check/steps/echo/files", tmpDir), ResultFile: fmt.Sprintf("%s/chapter_requirement_check/steps/echo/data.json", tmpDir), InputDirs: []string{fmt.Sprintf("%s/chapter_requirement_check/steps/write/files", tmpDir)}},
					},
					EvaluateResult: EvaluateResult{
						Results: []executor.Result{{
							Criterion:     "criteria1",
							Fulfilled:     true,
							Justification: "reason1",
							Metadata: map[string]string{
								"package":  "package1",
								"severity": "HIGH",
							},
						}},
						ExitCode: 0,
						ErrLogs:  nil,
						Logs: []string{
							`{"status": "GREEN", "reason": "file matches", "result": {"criterion": "criteria1", "fulfilled": true, "justification": "reason1", "metadata": {"severity": "HIGH", "package": "package1"}}}`,
						},
						Status: "GREEN",
						Reason: "file matches",
					},
					Name: "autopilot",
				}
			},
		},
		"should validate evaluate output and return error if invalid": {
			strict: true,
			check: &AutopilotCheck{
				Item: item,
				Autopilot: Autopilot{
					Name: "autopilot",
					Evaluate: Evaluate{
						Run: "echo '{\"reason\": \"hello world\"}';echo '{\"status\": \"GREEN\"}';",
					},
				},
			},
			want: func(tmpDir string) *AutopilotResult {
				return &AutopilotResult{
					EvaluateResult: EvaluateResult{
						ExitCode: 0,
						Logs: []string{
							`{"reason": "hello world"}`,
							`{"status": "GREEN"}`,
						},
						ErrLogs: nil,
						Reason:  "autopilot 'autopilot' did not provide any 'results'",
						Status:  "ERROR",
					},
					Name: "autopilot",
				}
			},
		},
		"should return some evaluate results if wrong data is passed": {
			strict: true,
			check: &AutopilotCheck{
				Item: item,
				Autopilot: Autopilot{
					Name: "autopilot",
					Evaluate: Evaluate{
						Run: "echo '{\"reason\": \"hello world\", \"status\": \"GREEN\",\"result\": {\"justification\": \"justified\", \"fulfilled\": true, \"criterion\": \"c1\", \"metadata\": {\"key\": \"value\"}}}'; echo '{\"result\": { \"metadata\": { key2: value }}}'",
					},
				},
			},
			want: func(tmpDir string) *AutopilotResult {
				return &AutopilotResult{
					EvaluateResult: EvaluateResult{
						ExitCode: 0,
						Logs: []string{
							`{"reason": "hello world", "status": "GREEN","result": {"justification": "justified", "fulfilled": true, "criterion": "c1", "metadata": {"key": "value"}}}`,
							`{"result": { "metadata": { key2: value }}}`,
						},
						Results: []executor.Result{
							{
								Metadata:      map[string]string{"key": "value"},
								Criterion:     "c1",
								Fulfilled:     true,
								Justification: "justified",
							},
						},
						ErrLogs: nil,
						Reason:  "hello world",
						Status:  "GREEN",
					},
					Name: "autopilot",
				}
			},
		},
		"should not strictly validate evaluate output if strict is false": {
			strict: false,
			check: &AutopilotCheck{
				Item: item,
				Autopilot: Autopilot{
					Name: "autopilot",
					Evaluate: Evaluate{
						Run: "echo '{\"reason\": \"hello world\"}';echo '{\"status\": \"GREEN\"}';",
					},
				},
			},
			want: func(tmpDir string) *AutopilotResult {
				return &AutopilotResult{
					EvaluateResult: EvaluateResult{
						ExitCode: 0,
						Logs: []string{
							`{"reason": "hello world"}`,
							`{"status": "GREEN"}`,
						},
						ErrLogs: nil,
						Reason:  "hello world",
						Status:  "GREEN",
					},
					Name: "autopilot",
				}
			},
		},
		"should error on evaluate timeout": {
			strict: false,
			check: &AutopilotCheck{
				Item: item,
				Autopilot: Autopilot{
					Name: "autopilot",
					Evaluate: Evaluate{
						Run: "sleep 11",
					},
				},
			},
			want: func(tmpDir string) *AutopilotResult {
				return &AutopilotResult{
					EvaluateResult: EvaluateResult{
						ExitCode: 124,
						Logs:     nil,
						ErrLogs: []string{
							"Command timed out after 10s",
						},
						Reason: "autopilot 'autopilot' timed out after 10s",
						Status: "ERROR",
					},
					Name: "autopilot",
				}
			},
		},
		"should not show secrets in logs": {
			strict: true,
			check: &AutopilotCheck{
				Item: item,
				Autopilot: Autopilot{
					Name:  "autopilot",
					Steps: [][]Step{{{Run: `echo "test_secret"`, ID: "test"}}},
					Evaluate: Evaluate{
						Run: "echo '{\"reason\": \"hello world\"}';echo '{\"status\": \"RED\"}';echo \"test_secret\"",
					},
				},
			},
			want: func(tmpDir string) *AutopilotResult {
				return &AutopilotResult{
					StepResults: []StepResult{{
						ID:        "test",
						ErrLogs:   nil,
						OutputDir: fmt.Sprintf("%s/chapter_requirement_check/steps/test/files", tmpDir),
						Logs: []string{
							"***TEST_SECRET***",
						}}},
					EvaluateResult: EvaluateResult{
						ExitCode: 0,
						Logs: []string{
							`{"reason": "hello world"}`,
							`{"status": "RED"}`,
							"***TEST_SECRET***",
						},
						ErrLogs: nil,
						Reason:  "autopilot 'autopilot' did not provide any 'results'",
						Status:  "ERROR",
					},
					Name: "autopilot",
				}
			},
		},
		"should handle validation errors": {
			strict: false,
			check: &AutopilotCheck{
				Item: item,
				Autopilot: Autopilot{
					Name: "autopilot",
				},
				ValidationErrs: []error{errors.New("validation error")},
			},
			want: func(tmpDir string) *AutopilotResult {
				return &AutopilotResult{
					EvaluateResult: EvaluateResult{
						Status:   "ERROR",
						Reason:   "autopilot 'autopilot' has the following validation errors and won't be executed: validation error",
						ExitCode: 0,
					},
					Name: "autopilot",
				}
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// arrange
			tmpDir := t.TempDir()

			logger := logger.NewAutopilot()
			timeout := 10 * time.Second
			secrets := map[string]string{"TEST_SECRET": "test_secret"}
			wdUtils := workdir.NewUtils(afero.NewOsFs())
			env := map[string]string{}

			// act
			actual, err := tc.check.Execute(wdUtils, tmpDir, env, secrets, tc.strict, *logger, timeout, runner.NewSubprocess(logger))
			expected := tc.want(tmpDir)

			// assert
			assert.NotNil(t, actual)
			assert.NoError(t, err)
			assert.Equal(t, expected, actual)
		})
	}
}

func TestAutopilotExecuteDirectoryStructure(t *testing.T) {
	item :=
		Item{
			Chapter: configuration.Chapter{
				Id: "chapter",
			},
			Requirement: configuration.Requirement{
				Id: "requirement",
			},
			Check: configuration.Check{
				Id: "check",
			},
		}
	testCases := map[string]struct {
		check  *AutopilotCheck
		strict bool
		want   map[string][]string
	}{
		"should not create steps directory validation errors exist": {
			check: &AutopilotCheck{
				Item:           item,
				ValidationErrs: []error{errors.New("some error")},
				Autopilot: Autopilot{
					Name:     "autopilot",
					Steps:    [][]Step{},
					Evaluate: Evaluate{},
				},
			},
			strict: true,
			want:   map[string][]string{},
		},
		"should not create steps directory if there are no steps": {
			check: &AutopilotCheck{
				Item: item,
				Autopilot: Autopilot{
					Name:     "autopilot",
					Steps:    [][]Step{},
					Evaluate: Evaluate{},
				},
			},
			strict: false,
			want: map[string][]string{
				"chapter_requirement_check":            {},
				"chapter_requirement_check/evaluation": {},
			},
		},
		"should create steps with correct files": {
			check: &AutopilotCheck{
				Item: item,
				Autopilot: Autopilot{
					Name: "autopilot",
					Steps: [][]Step{{
						Step{
							Title: "write hello word",
							Configs: map[string]string{
								"config1": "value1",
								"config2": "value2",
							},
							Depends: []string{},
							ID:      "write",
							Run:     "echo '{\"key\": \"hello world\"}' > $AUTOPILOT_OUTPUT_DIR/data.txt\necho 'done writing'",
						},
						Step{
							Title: "say hello word",
							Configs: map[string]string{
								"config1": "value1",
								"config2": "value2",
							},
							Depends: []string{"write"},
							ID:      "echo",
							Run:     "cat $AUTOPILOT_INPUT_DIRS/data.txt > $AUTOPILOT_RESULT_FILE\necho 'done echoing'",
						},
					}},
					Env: map[string]string{
						"ENV_VAR1": "value1",
						"ENV_VAR2": "value2",
					},
					Evaluate: Evaluate{
						Run: "data=$(cat \"$EVALUATOR_INPUT_FILES\"); expected='{\"key\": \"hello world\"}'; [[ \"$data\" == \"$expected\" ]] && echo 'hello world'; echo '{\"status\": \"GREEN\", \"reason\": \"file matches\", \"result\": {\"criterion\": \"criteria1\", \"fulfilled\": true, \"justification\": \"reason1\", \"metadata\": {\"severity\": \"HIGH\", \"package\": \"package1\"}}}' > $EVALUATOR_RESULT_FILE || echo 'hello world'; echo '{\"status\": \"RED\", \"reason\": \"file does not matches\", \"result\": {\"criterion\": \"criteria1\", \"fulfilled\": false, \"justification\": \"reason1\", \"metadata\": {\"severity\": \"HIGH\", \"package\": \"package1\"}}}' > $EVALUATOR_RESULT_FILE",
					},
				},
			},
			strict: false,
			want: map[string][]string{
				"chapter_requirement_check":                   {},
				"chapter_requirement_check/steps":             {},
				"chapter_requirement_check/steps/write":       {"logs.txt"},
				"chapter_requirement_check/steps/write/work":  {"config1", "config2"},
				"chapter_requirement_check/steps/write/files": {"data.txt"},
				"chapter_requirement_check/steps/echo":        {"logs.txt", "data.json"},
				"chapter_requirement_check/steps/echo/work":   {"config1", "config2"},
				"chapter_requirement_check/steps/echo/files":  {},
				"chapter_requirement_check/evaluation":        {"logs.txt", "result.json"},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// arrange
			tmpDir := t.TempDir()

			logger := logger.NewAutopilot()
			timeout := 10 * time.Second
			secrets := map[string]string{}
			wdUtils := workdir.NewUtils(afero.NewOsFs())
			env := map[string]string{}

			// act
			actual, err := tc.check.Execute(wdUtils, tmpDir, env, secrets, tc.strict, *logger, timeout, runner.NewSubprocess(logger))

			// assert
			assert.NotNil(t, actual)
			assert.NoError(t, err)

			// make sure expected files were created
			for dir, files := range tc.want {
				for _, file := range files {
					filePath := filepath.Join(tmpDir, dir, file)
					_, err := os.Stat(filePath)
					assert.NoError(t, err)
				}
			}

			// make sure no extra files or directories where created
			err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				relPath, err := filepath.Rel(tmpDir, path)
				if err != nil {
					return err
				}

				if relPath == "." {
					return nil
				}

				if info.IsDir() {
					found := false
					for dir := range tc.want {
						if relPath == dir {
							found = true
							break
						}
					}

					if !found {
						t.Errorf("an unexpected directory %s was created", relPath)
					}
				} else {
					found := false
					for dir, files := range tc.want {
						for _, file := range files {
							expectedPath := filepath.Join(dir, file)
							if relPath == expectedPath {
								found = true
								break
							}
						}
					}
					if !found {
						t.Errorf("an unexpected file %s was created", relPath)
					}
				}
				return nil
			})

			assert.NoError(t, err)
		})
	}
}
