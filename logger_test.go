package sentinel

import "testing"

func TestInfoLog(t *testing.T) {
	logger.Debug("debug")
	logger.Warn("warn")
	logger.Info("info")
	logger.Error("error")

	logger.Debugf("%v", "debug")
	logger.Warnf("%v", "warn")
	logger.Infof("%v", "info")
	logger.Errorf("%v", "error")

	logger.SetLevel(LOG_ERR)
	logger.Debug("debug")
	logger.Warn("warn")
	logger.Info("info")
	logger.Error("error")

	logger.Debugf("%v", "debug")
	logger.Warnf("%v", "warn")
	logger.Infof("%v", "info")
	logger.Errorf("%v", "error")
}
