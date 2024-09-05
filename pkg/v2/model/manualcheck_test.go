package model

import (
	"testing"

	"github.com/B-S-F/onyx/pkg/configuration"
	"github.com/B-S-F/onyx/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestManualExecuteIntegration(t *testing.T) {
	item := &ManualCheck{
		Item: Item{
			Chapter: configuration.Chapter{
				Id: "chapter",
			},
			Requirement: configuration.Requirement{
				Id: "requirement",
			},
			Check: configuration.Check{
				Id: "check",
			},
		},
		Manual: configuration.Manual{
			Status: "GREEN",
			Reason: "completed manually",
		},
	}
	t.Run("should return output", func(t *testing.T) {
		// arrange
		logger := logger.NewAutopilot()

		// act
		output, err := item.Execute(logger)

		// assert
		assert.NotNil(t, output)
		assert.NoError(t, err)
		assert.Equal(t, "GREEN", output.Status)
		assert.Equal(t, "completed manually", output.Reason)
	})
}
