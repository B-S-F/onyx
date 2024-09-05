package model

import (
	"path/filepath"
	"time"

	"github.com/B-S-F/onyx/pkg/helper"
	"github.com/B-S-F/onyx/pkg/logger"
	"github.com/B-S-F/onyx/pkg/runner"
	"github.com/B-S-F/onyx/pkg/v2/executor"
	"github.com/B-S-F/onyx/pkg/workdir"
	"github.com/pkg/errors"
)

type Finalize struct {
	Env     map[string]string
	Configs map[string]string
	Run     string
}

type FinalizeResult struct {
	Logs       []string
	ErrLogs    []string
	ExitCode   int
	OutputPath string
}

func (item *Finalize) Execute(wdUtils workdir.Utilizer, rootWorkDir string, env, secrets map[string]string, logger logger.Autopilot, timeout time.Duration, runner runner.Runner) (*FinalizeResult, error) {
	err := item.overWriteConfigFiles(wdUtils, item.Configs, rootWorkDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create config files")
	}
	specialEnv := map[string]string{"result_path": rootWorkDir}
	runtimeEnv := helper.MergeMaps(env, item.Env, specialEnv)
	runnerOutput, err := executor.StartRunner(rootWorkDir, item.Run, runtimeEnv, secrets, logger, runner, timeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run finalize")
	}
	result := &FinalizeResult{
		Logs:       runnerOutput.Logs,
		ErrLogs:    runnerOutput.ErrLogs,
		ExitCode:   runnerOutput.ExitCode,
		OutputPath: runnerOutput.WorkDir,
	}
	output := executor.Output{
		Logs:     runnerOutput.Logs,
		ErrLogs:  runnerOutput.ErrLogs,
		ExitCode: runnerOutput.ExitCode,
	}
	output.Log(&logger)
	return result, nil
}

func (f *Finalize) overWriteConfigFiles(wdUtils workdir.Utilizer, configs map[string]string, workDir string) error {
	for file, content := range configs {
		err := wdUtils.UpdateContentForce(filepath.Join(workDir, file), []byte(content))
		if err != nil {
			return errors.Wrapf(err, "failed to overwrite config file %s", file)
		}
	}
	return nil
}
