package main

import (
	"encoding/json"
	"os"

	"github.com/mcpx/boilerplate/services"
)

func main() {
	result := runJudgeWorker()
	_ = json.NewEncoder(os.Stdout).Encode(result)
}

func runJudgeWorker() services.JudgeWorkerResult {
	encodedPayload := os.Getenv(services.JudgeWorkerPayloadEnv)

	runnerInput, err := services.DecodeJudgeWorkerPayload(encodedPayload)
	if err != nil {
		return services.JudgeWorkerResult{Error: err.Error()}
	}

	localRunner := &services.LocalRunner{WorkDir: os.Getenv("JUDGE_WORKDIR")}
	runnerOutput, err := localRunner.Run(runnerInput)
	if err != nil {
		return services.JudgeWorkerResult{Error: err.Error()}
	}

	return services.JudgeWorkerResult{RunnerOutput: &runnerOutput}
}
