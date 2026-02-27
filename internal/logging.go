package internal

import (
	"fmt"
	"log"
	"os"
)

var logger = log.New(os.Stderr, "", log.LstdFlags)

func Logf(format string, args ...any) {
	logger.Output(2, fmt.Sprintf(format, args...))
}

func Errorf(format string, args ...any) {
	logger.Output(2, "ERROR: "+fmt.Sprintf(format, args...))
}
