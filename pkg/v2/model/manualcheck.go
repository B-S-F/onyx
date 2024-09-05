package model

import (
	conf "github.com/B-S-F/onyx/pkg/configuration"
	"github.com/B-S-F/onyx/pkg/logger"
	"github.com/B-S-F/onyx/pkg/v2/executor"
)

type ManualCheck struct {
	Item
	Manual conf.Manual
}

type ManualResult struct {
	Status string
	Reason string
}

func (item *ManualCheck) Execute(logger *logger.Autopilot) (*ManualResult, error) {
	logger.Info("providing manual answer")
	result := &ManualResult{
		Status: item.Manual.Status,
		Reason: item.Manual.Reason,
	}
	output := executor.Output{
		Reason: item.Manual.Reason,
		Status: item.Manual.Status,
	}
	output.Log(logger)
	return result, nil
}
