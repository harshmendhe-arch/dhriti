package main

import (
	"github.com/opencode-ai/dhriti/cmd"
	"github.com/opencode-ai/dhriti/internal/logging"
)

func main() {
	defer logging.RecoverPanic("main", func() {
		logging.ErrorPersist("Application terminated due to unhandled panic")
	})

	cmd.Execute()
}
