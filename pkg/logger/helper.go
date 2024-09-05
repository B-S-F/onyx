package logger

import (
	"fmt"
	"strings"
)

type LogHelper struct {
	logger Logger
}

func NewHelper(logger Logger) *LogHelper {
	return &LogHelper{
		logger: logger,
	}
}

func (h *LogHelper) LogKeyValueIndented(key string, value string, indentation ...int) {
	if len(key) == 0 && len(value) == 0 {
		return
	}
	indent := 2
	if len(indentation) != 0 {
		indent = indentation[0]
	}
	msg := fmt.Sprintf("%s%s %s", strings.Repeat(" ", indent), key, value)
	msg = strings.TrimSuffix(msg, " ")
	h.logger.Info(msg)
}

func (h *LogHelper) LogFormatMapIndented(key string, value map[string]string, indentation ...int) {
	if len(key) == 0 && len(value) == 0 {
		return
	}
	indent := 2
	if len(indentation) != 0 {
		indent = indentation[0]
	}
	h.logger.Info(fmt.Sprintf("%s%s", strings.Repeat(" ", indent), key))
	for k, v := range value {
		h.logger.Info(fmt.Sprintf("%s%s: %s", strings.Repeat(" ", indent+2), k, v))
	}
}
