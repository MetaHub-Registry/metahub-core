package logging

import (
	graylog "github.com/gemnasium/logrus-graylog-hook/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// InitLog is the interface that wraps the basic Log method.
func InitLog() {
	lLevel := viper.GetString("logging.level")
	switch lLevel {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	case "panic":
		logrus.SetLevel(logrus.PanicLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}
	gelf := viper.GetString("logging.gelf")
	if gelf != "" {
		logrus.Infof("Send logs to Graylog: %s", gelf)
		hook := graylog.NewGraylogHook(gelf, map[string]interface{}{})
		logrus.AddHook(hook)
	}
}
