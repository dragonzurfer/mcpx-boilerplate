# docs.md

## Purpose

Standalone process that runs one practice submission inside a Kubernetes Job pod.

## Files

- `main.go`: decodes the runner payload from `JUDGE_RUNNER_PAYLOAD_B64`, executes `services.LocalRunner`, and prints JSON result for the orchestrator.

