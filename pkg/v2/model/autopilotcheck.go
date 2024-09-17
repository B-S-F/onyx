package model

import (
	"encoding/json"
	errs "errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	conf "github.com/B-S-F/onyx/pkg/configuration"
	"github.com/B-S-F/onyx/pkg/helper"
	"github.com/B-S-F/onyx/pkg/logger"
	"github.com/B-S-F/onyx/pkg/runner"
	"github.com/B-S-F/onyx/pkg/v2/executor"
	"github.com/B-S-F/onyx/pkg/workdir"
	"github.com/chigopher/pathlib"
	"github.com/pkg/errors"
)

type AutopilotCheck struct {
	Item
	Autopilot      Autopilot
	CheckEnv       map[string]string
	AppReferences  []*conf.AppReference
	ValidationErrs []error
	AppPath        string
}

type StepResult struct {
	ID         string
	OutputDir  string
	ResultFile string
	Logs       []string
	ErrLogs    []string
	ExitCode   int
	InputDirs  []string
}
type EvaluateResult struct {
	Logs     []string
	ErrLogs  []string
	ExitCode int
	Status   string
	Reason   string
	Results  []executor.Result
}

type AutopilotResult struct {
	StepResults    []StepResult
	EvaluateResult EvaluateResult
	Name           string
}

func (item *AutopilotCheck) Execute(wdUtils workdir.Utilizer, rootWorkDir string, env, secrets map[string]string, strict bool, logger logger.Autopilot, timeout time.Duration, runner runner.Runner) (*AutopilotResult, error) {
	if result := item.checkErrors(&logger); result != nil {
		return result, nil
	}

	// setup
	sysPATH := os.Getenv("PATH")
	if item.AppPath != "" {
		sysPATH = fmt.Sprintf("%s:%s", item.AppPath, sysPATH)
	}
	checkUid := strings.Join([]string{item.Chapter.Id, item.Requirement.Id, item.Check.Id}, "_")
	checkDir, err := wdUtils.CreateDir(rootWorkDir, checkUid)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to create check directory for check '%s'", checkUid))
	}
	var stepsDir *pathlib.Path
	if len(item.Autopilot.Steps) > 0 {
		stepsDir, err = wdUtils.CreateDir(checkDir.String(), "steps")
		if err != nil {
			return nil, errors.Wrap(err, fmt.Sprintf("failed to create steps directory for check '%s'", checkUid))
		}
	}
	var stepResults []StepResult
	for _, stepsLevel := range item.Autopilot.Steps {
		for _, step := range stepsLevel {
			// prepare directory structure
			stepDirs, err := item.prepareStepDirs(wdUtils, stepsDir.String(), step.ID)
			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("failed to create step directories for step '%s'", step.ID))
			}
			// create specified configuration files
			err = item.createConfigFiles(wdUtils, step.Configs, stepDirs.workDir)
			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("failed to create config files for step '%s'", step.ID))
			}
			// link required files
			err = wdUtils.LinkFiles(rootWorkDir, stepDirs.workDir)
			defer wdUtils.RemoveLinkedFiles(stepDirs.workDir)
			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("failed to link files for step '%s'", step.ID))
			}
			// prepare input directories
			var inputDirs []string
			for _, depend := range step.Depends {
				dependDir := filepath.Join(stepsDir.String(), depend, "files")
				if _, err := os.Stat(dependDir); os.IsNotExist(err) {
					return nil, errors.Wrap(err, fmt.Sprintf("step '%s' depends on '%s' but the step doesn't exist or didn't execute properly", step.ID, depend))
				}
				inputDirs = append(inputDirs, dependDir)
			}
			// prepare environment variables
			specialEnv := map[string]string{
				"APPS":                  item.AppPath,
				"PATH":                  sysPATH,
				"AUTOPILOT_OUTPUT_DIR":  stepDirs.filesDir,
				"AUTOPILOT_INPUT_DIRS":  strings.Join(inputDirs, strconv.QuoteRune(os.PathListSeparator)),
				"AUTOPILOT_RESULT_FILE": filepath.Join(stepDirs.stepDir, "data.json"),
			}
			runtimeEnv := helper.MergeMaps(env, step.Env, item.Autopilot.Env, specialEnv)
			// do run
			logger.Info(fmt.Sprintf("starting autopilot '%s' step '%s'", item.Autopilot.Name, step.ID))
			runnerOutput, err := executor.StartRunner(stepDirs.workDir, step.Run, runtimeEnv, secrets, logger, runner, timeout)
			if err != nil {
				return nil, errors.Wrap(err, fmt.Sprintf("failed to run autopilot '%s' step '%s'", item.Autopilot.Name, step.ID))
			}
			// get step result and log output
			stepResult := item.parseStepResult(runnerOutput, step.ID, stepDirs, inputDirs)
			if err := writeLogs(stepDirs.stepDir, wdUtils, stepResult.Logs, stepResult.ErrLogs); err != nil {
				logger.Info(fmt.Sprintf("couldn't write logs for autopilot '%s' step '%s'", item.Autopilot.Name, step.ID))
			}
			stepResults = append(stepResults, stepResult)
		}
	}

	// do evaluation
	evalDir, err := wdUtils.CreateDir(checkDir.String(), "evaluation")
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to create evaluation directory '%s'", evalDir))
	}
	err = item.createConfigFiles(wdUtils, item.Autopilot.Evaluate.Configs, evalDir.String())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create configuration files for evaluation")
	}
	var evalInputFiles []string
	for _, step := range stepResults {
		dataFile := step.ResultFile
		if _, err := os.Stat(dataFile); err == nil {
			evalInputFiles = append(evalInputFiles, dataFile)
		}
	}
	specialEnv := map[string]string{
		"PATH":                  sysPATH,
		"EVALUATOR_INPUT_FILES": strings.Join(evalInputFiles, strconv.QuoteRune(os.PathListSeparator)),
		"EVALUATOR_RESULT_FILE": filepath.Join(evalDir.String(), "result.json"),
	}
	runtimeEnv := helper.MergeMaps(env, item.Autopilot.Evaluate.Env, specialEnv)
	logger.Info("doing evaluation")
	evalOutput, err := executor.StartRunner(evalDir.String(), item.Autopilot.Evaluate.Run, runtimeEnv, secrets, logger, runner, timeout)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to run autopilot '%s' evaluation", item.Autopilot.Name))
	}
	if len(evalOutput.Logs) > 0 {
		if err := writeLogs(evalDir.String(), wdUtils, evalOutput.Logs, evalOutput.ErrLogs); err != nil {
			logger.Warn(fmt.Sprintf("failed to write logs for autopilot '%s' evaluation", item.Autopilot.Name))
		}
	}
	evalResult, err := item.parseEvaluatorResult(evalOutput)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse evaluate result")
	}
	autopilotResult := &AutopilotResult{
		StepResults: stepResults,
		EvaluateResult: EvaluateResult{
			ExitCode: evalOutput.ExitCode,
			Logs:     evalOutput.Logs,
			ErrLogs:  evalOutput.ErrLogs,
			Results:  evalResult.results,
			Status:   evalResult.status,
			Reason:   evalResult.reason,
		},
		Name: item.Autopilot.Name,
	}
	checkResult(autopilotResult, strict, timeout, &logger)
	output := executor.Output{
		ExitCode:     autopilotResult.EvaluateResult.ExitCode,
		Logs:         autopilotResult.EvaluateResult.Logs,
		ErrLogs:      autopilotResult.EvaluateResult.ErrLogs,
		EvidencePath: checkDir.String(),
		Status:       autopilotResult.EvaluateResult.Status,
		Reason:       autopilotResult.EvaluateResult.Reason,
		Results:      autopilotResult.EvaluateResult.Results,
		Name:         autopilotResult.Name,
	}
	output.Log(&logger)
	return autopilotResult, nil
}

func checkResult(result *AutopilotResult, strict bool, timeout time.Duration, logger *logger.Autopilot) {
	if result.EvaluateResult.ExitCode != 0 {
		var msg string
		if result.EvaluateResult.ExitCode == 124 {
			msg = fmt.Sprintf("autopilot '%s' timed out after %s", result.Name, timeout)
		} else {
			msg = fmt.Sprintf("autopilot '%s' exited with exit code %d", result.Name, result.EvaluateResult.ExitCode)
		}
		result.EvaluateResult.Status = "ERROR"
		result.EvaluateResult.Reason = msg
		logger.Error(msg)
		return
	}
	// autopilot must provide a status of RED, GREEN, YELLOW
	allowedStatus := []string{"RED", "GREEN", "YELLOW"}
	if !helper.Contains(allowedStatus, result.EvaluateResult.Status) {
		msg := fmt.Sprintf("autopilot '%s' provided an invalid 'status': '%s'", result.Name, result.EvaluateResult.Status)
		result.EvaluateResult.Status = "ERROR"
		result.EvaluateResult.Reason = msg
		logger.Error(msg)
		return
	}
	// autopilot must provide a reason
	var msgs []string
	if result.EvaluateResult.Reason == "" {
		msgs = append(msgs, fmt.Sprintf("autopilot '%s' did not provide a 'reason'", result.Name))
	}
	// autopilot with status RED, GREEN, YELLOW must provide results
	if result.EvaluateResult.Results == nil || len(result.EvaluateResult.Results) == 0 {
		msgs = append(msgs, fmt.Sprintf("autopilot '%s' did not provide any 'results'", result.Name))
	}
	// autopilot must provide a criterion and justification for each result
	for i, r := range result.EvaluateResult.Results {
		if r.Criterion == "" {
			msgs = append(msgs, fmt.Sprintf("autopilot '%s' did not provide a 'criterion' in result '%d'", result.Name, i))
		}
		if r.Justification == "" {
			msgs = append(msgs, fmt.Sprintf("autopilot '%s' did not provide a 'justification' in result '%d'", result.Name, i))
		}
	}
	if len(msgs) == 0 {
		return
	}
	msg := strings.Join(msgs, "; ")
	if strict {
		result.EvaluateResult.Status = "ERROR"
		result.EvaluateResult.Reason = msg
		logger.Error(msg)
		return
	} else {
		logger.Warn(msg)
	}
}

type evaluateResult struct {
	status  string
	reason  string
	results []executor.Result
}

func (item *AutopilotCheck) checkErrors(logger *logger.Autopilot) *AutopilotResult {
	if len(item.ValidationErrs) > 0 {
		msg := fmt.Sprintf("autopilot '%s' has the following validation errors and won't be executed: %s", item.Autopilot.Name, errs.Join(item.ValidationErrs...).Error())
		output := &AutopilotResult{
			EvaluateResult: EvaluateResult{
				ExitCode: 0,
				Status:   "ERROR",
				Reason:   msg,
			},
			Name: item.Autopilot.Name,
		}
		logger.Error(msg)
		return output
	}
	return nil
}

type stepDirs struct {
	stepDir  string
	workDir  string
	filesDir string
}

func (item *AutopilotCheck) prepareStepDirs(wdUtils workdir.Utilizer, stepsDir, stepID string) (*stepDirs, error) {
	stepDir, err := wdUtils.CreateDir(stepsDir, stepID)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to create step directory for step '%s'", stepID))
	}
	workDir, err := wdUtils.CreateDir(stepDir.String(), "work")
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to create work directory for step '%s'", stepID))
	}
	outputDir, err := wdUtils.CreateDir(stepDir.String(), "files")
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to create output directory for step '%s'", stepID))
	}
	return &stepDirs{
		workDir:  workDir.String(),
		filesDir: outputDir.String(),
		stepDir:  stepDir.String(),
	}, nil
}

func (item *AutopilotCheck) createConfigFiles(wdUtils workdir.Utilizer, config map[string]string, workDir string) error {
	for file, content := range config {
		err := wdUtils.CreateFile(filepath.Join(workDir, file), []byte(content))
		if err != nil {
			return errors.Wrapf(err, "failed to write configuration file '%s'", file)
		}
	}
	return nil
}

func (item *AutopilotCheck) parseStepResult(runnerOutput *runner.Output, id string, stepDirs *stepDirs, inputDirs []string) StepResult {
	result := StepResult{
		ID:        id,
		OutputDir: stepDirs.filesDir,
		Logs:      runnerOutput.Logs,
		ErrLogs:   runnerOutput.ErrLogs,
		InputDirs: inputDirs,
		ExitCode:  runnerOutput.ExitCode,
	}
	resultFile := filepath.Join(stepDirs.stepDir, "data.json")
	if _, err := os.Stat(resultFile); err == nil {
		result.ResultFile = resultFile
	}
	return result
}

func (item *AutopilotCheck) parseEvaluatorResult(runnerOutput *runner.Output) (*evaluateResult, error) {
	out := &evaluateResult{}
	for _, data := range runnerOutput.Data {
		if status, ok := data["status"].(string); ok {
			out.status = status
		}

		if reason, ok := data["reason"].(string); ok {
			out.reason = reason
		}
		if results, ok := data["result"].(map[string]interface{}); ok {
			r := executor.Result{}
			resultMap := results
			if criteria, ok := resultMap["criterion"].(string); ok {
				r.Criterion = criteria
			}
			if fulfilled, ok := resultMap["fulfilled"].(bool); ok {
				r.Fulfilled = fulfilled
			}
			if justification, ok := resultMap["justification"].(string); ok {
				r.Justification = justification
			}
			if metadata, ok := resultMap["metadata"].(map[string]interface{}); ok {
				dataMap := make(map[string]string)
				for k, v := range metadata {
					switch v.(type) {
					// All complex objects are reverted to fit into the string format to match the expected output
					case map[string]interface{}:
						marshaled, err := json.Marshal(v)
						if err != nil {
							return nil, err
						}
						dataMap[k] = string(marshaled)
					default:
						dataMap[k] = fmt.Sprintf("%v", v)
					}
				}
				if len(dataMap) > 0 {
					r.Metadata = dataMap
				}
			}
			out.results = append(out.results, r)
		}
	}
	return out, nil
}
func writeLogs(workDir string, wdUtils workdir.Utilizer, logs []string, errLogs []string) error {
	combinedLogs := append(logs, errLogs...)
	if err := wdUtils.CreateFile(filepath.Join(workDir, "logs.txt"), []byte(strings.Join(combinedLogs, "\n"))); err != nil {
		return err
	}
	return nil
}
