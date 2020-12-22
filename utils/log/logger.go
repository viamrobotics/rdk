package log

import "go.uber.org/zap"

type Logger = *zap.SugaredLogger

var Global Logger

func init() {
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	Global = logger.Sugar()
}
