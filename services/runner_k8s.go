package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type K8sJobRunner struct {
	Namespace      string
	Image          string
	ServiceAccount string
	CPULimit       string
	PollInterval   time.Duration
	JobTTLSeconds  int32
	KubectlBin     string
	KubeconfigPath string
}

type k8sCreateJobInput struct {
	RunnerInput RunnerInput
	PayloadB64  string
	JobName     string
	JobTimeout  time.Duration
}

type k8sPollResult struct {
	PodName string
	Phase   string
}

type k8sJobTimeoutInput struct {
	ExecutionLimits ExecutionLimits
	TestCount       int
}

type kubectlRunInput struct {
	Context context.Context
	Args    []string
	Stdin   string
}

type jobPod struct {
	Name  string
	Phase string
}

func (r *K8sJobRunner) Run(input RunnerInput) (RunnerOutput, error) {
	image := strings.TrimSpace(r.Image)
	if image == "" {
		return RunnerOutput{}, errors.New("JUDGE_K8S_IMAGE is required for k8s runner")
	}

	encodedPayload, err := EncodeJudgeWorkerPayload(input)
	if err != nil {
		return RunnerOutput{}, err
	}

	jobName := buildK8sJudgeJobName(input.SubmissionID)
	jobTimeout := resolveK8sJobTimeout(k8sJobTimeoutInput{
		ExecutionLimits: input.Limits,
		TestCount:       len(input.Tests),
	})

	if err := r.createJob(k8sCreateJobInput{
		RunnerInput: input,
		PayloadB64:  encodedPayload,
		JobName:     jobName,
		JobTimeout:  jobTimeout,
	}); err != nil {
		return RunnerOutput{}, err
	}
	defer r.deleteJob(jobName)

	pollContext, cancel := context.WithTimeout(context.Background(), jobTimeout+15*time.Second)
	defer cancel()

	pollResult, err := r.waitForJobCompletion(pollContext, jobName)
	if err != nil {
		return RunnerOutput{}, err
	}

	podLogs, err := r.fetchPodLogs(pollContext, pollResult.PodName)
	if err != nil {
		return RunnerOutput{}, err
	}

	workerResult, err := parseJudgeWorkerResult([]byte(podLogs))
	if err != nil {
		return RunnerOutput{}, err
	}

	if workerResult.Error != "" {
		return RunnerOutput{}, errors.New(workerResult.Error)
	}

	if workerResult.RunnerOutput == nil {
		return RunnerOutput{}, errors.New("judge worker result missing runner_output")
	}

	return *workerResult.RunnerOutput, nil
}

func (r *K8sJobRunner) createJob(input k8sCreateJobInput) error {
	jobManifest, err := r.buildJobManifest(input)
	if err != nil {
		return err
	}

	commandContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err = r.runKubectl(kubectlRunInput{
		Context: commandContext,
		Args: []string{
			"-n", r.resolveNamespace(),
			"apply",
			"-f", "-",
		},
		Stdin: jobManifest,
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *K8sJobRunner) buildJobManifest(input k8sCreateJobInput) (string, error) {
	activeDeadlineSeconds := int64(input.JobTimeout.Seconds())
	if activeDeadlineSeconds < 1 {
		activeDeadlineSeconds = 1
	}

	resourceSpec := map[string]any{
		"requests": map[string]string{
			"cpu":    r.resolveCPULimit(),
			"memory": resolveK8sMemoryLimit(input.RunnerInput.Limits.MemoryLimitKb),
		},
		"limits": map[string]string{
			"cpu":    r.resolveCPULimit(),
			"memory": resolveK8sMemoryLimit(input.RunnerInput.Limits.MemoryLimitKb),
		},
	}

	securityContext := map[string]any{
		"runAsNonRoot":             true,
		"allowPrivilegeEscalation": false,
		"capabilities": map[string][]string{
			"drop": {"ALL"},
		},
	}

	containerSpec := map[string]any{
		"name":            "judge-worker",
		"image":           strings.TrimSpace(r.Image),
		"imagePullPolicy": "IfNotPresent",
		"command":         []string{"/app/judge-worker"},
		"env": []map[string]string{
			{"name": JudgeWorkerPayloadEnv, "value": input.PayloadB64},
			{"name": "HOME", "value": "/tmp"},
			{"name": "TMPDIR", "value": "/tmp"},
			{"name": "GOCACHE", "value": "/tmp/go-build-cache"},
		},
		"resources":       resourceSpec,
		"securityContext": securityContext,
	}

	podSpec := map[string]any{
		"restartPolicy":                "Never",
		"automountServiceAccountToken": false,
		"containers":                   []map[string]any{containerSpec},
	}

	serviceAccount := strings.TrimSpace(r.ServiceAccount)
	if serviceAccount != "" {
		podSpec["serviceAccountName"] = serviceAccount
	}

	jobSpec := map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]any{
			"name":      input.JobName,
			"namespace": r.resolveNamespace(),
			"labels": map[string]string{
				"app.kubernetes.io/component": "judge-runner",
				"judge-submission-id":         strconv.FormatUint(uint64(input.RunnerInput.SubmissionID), 10),
			},
		},
		"spec": map[string]any{
			"backoffLimit":            0,
			"ttlSecondsAfterFinished": r.resolveJobTTLSeconds(),
			"activeDeadlineSeconds":   activeDeadlineSeconds,
			"template": map[string]any{
				"metadata": map[string]any{
					"labels": map[string]string{
						"app.kubernetes.io/component": "judge-runner",
					},
				},
				"spec": podSpec,
			},
		},
	}

	rawManifest, err := json.Marshal(jobSpec)
	if err != nil {
		return "", err
	}

	return string(rawManifest), nil
}

func (r *K8sJobRunner) waitForJobCompletion(
	pollContext context.Context,
	jobName string,
) (k8sPollResult, error) {
	for {
		if err := pollContext.Err(); err != nil {
			return k8sPollResult{}, err
		}

		jobPod, err := r.fetchJobPod(pollContext, jobName)
		if err != nil {
			return k8sPollResult{}, err
		}

		if jobPod != nil {
			if jobPod.Phase == "Succeeded" || jobPod.Phase == "Failed" {
				return k8sPollResult{PodName: jobPod.Name, Phase: jobPod.Phase}, nil
			}
		}

		time.Sleep(r.resolvePollInterval())
	}
}

func (r *K8sJobRunner) fetchJobPod(
	commandContext context.Context,
	jobName string,
) (*jobPod, error) {
	output, err := r.runKubectl(kubectlRunInput{
		Context: commandContext,
		Args: []string{
			"-n", r.resolveNamespace(),
			"get", "pods",
			"-l", "job-name=" + jobName,
			"-o", "json",
		},
	})
	if err != nil {
		return nil, err
	}

	type podList struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}

	parsedPodList := podList{}
	if err := json.Unmarshal([]byte(output), &parsedPodList); err != nil {
		return nil, err
	}

	if len(parsedPodList.Items) == 0 {
		return nil, nil
	}

	firstPod := parsedPodList.Items[0]
	return &jobPod{Name: firstPod.Metadata.Name, Phase: firstPod.Status.Phase}, nil
}

func (r *K8sJobRunner) fetchPodLogs(
	commandContext context.Context,
	podName string,
) (string, error) {
	return r.runKubectl(kubectlRunInput{
		Context: commandContext,
		Args: []string{
			"-n", r.resolveNamespace(),
			"logs", podName,
			"-c", "judge-worker",
		},
	})
}

func parseJudgeWorkerResult(rawLogs []byte) (JudgeWorkerResult, error) {
	trimmedLogs := strings.TrimSpace(string(rawLogs))
	if trimmedLogs == "" {
		return JudgeWorkerResult{}, errors.New("judge worker emitted empty output")
	}

	workerResult := JudgeWorkerResult{}
	if err := json.Unmarshal([]byte(trimmedLogs), &workerResult); err != nil {
		return JudgeWorkerResult{}, fmt.Errorf("invalid judge worker output: %w", err)
	}

	return workerResult, nil
}

func (r *K8sJobRunner) deleteJob(jobName string) {
	deleteContext, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	_, _ = r.runKubectl(kubectlRunInput{
		Context: deleteContext,
		Args: []string{
			"-n", r.resolveNamespace(),
			"delete", "job", jobName,
			"--ignore-not-found=true",
			"--wait=false",
		},
	})
}

func buildK8sJudgeJobName(submissionID uint) string {
	timestamp := time.Now().UTC().UnixNano()
	return fmt.Sprintf("judge-sub-%d-%d", submissionID, timestamp)
}

func resolveK8sMemoryLimit(memoryLimitKb int) string {
	memoryLimitBytes := int64(memoryLimitKb) * 1024
	if memoryLimitBytes <= 0 {
		memoryLimitBytes = 256 * 1024 * 1024
	}

	minimumBytes := int64(128 * 1024 * 1024)
	if memoryLimitBytes < minimumBytes {
		memoryLimitBytes = minimumBytes
	}

	memoryLimitMi := memoryLimitBytes / (1024 * 1024)
	return strconv.FormatInt(memoryLimitMi, 10) + "Mi"
}

func resolveK8sJobTimeout(input k8sJobTimeoutInput) time.Duration {
	compileTimeout := resolveJobCompileTimeout(input.ExecutionLimits.TimeLimitMs)
	runTimeout := resolveTimeLimit(input.ExecutionLimits.TimeLimitMs)

	testCount := input.TestCount
	if testCount <= 0 {
		testCount = 1
	}

	baseTimeout := compileTimeout + (runTimeout * time.Duration(testCount)) + 10*time.Second
	if baseTimeout < 30*time.Second {
		return 30 * time.Second
	}
	if baseTimeout > 10*time.Minute {
		return 10 * time.Minute
	}

	return baseTimeout
}

func resolveJobCompileTimeout(timeLimitMs int) time.Duration {
	runtimeLimit := resolveTimeLimit(timeLimitMs)
	compileLimit := runtimeLimit * 2
	if compileLimit < 3*time.Second {
		return 3 * time.Second
	}
	if compileLimit > 90*time.Second {
		return 90 * time.Second
	}

	return compileLimit
}

func (r *K8sJobRunner) resolveNamespace() string {
	namespace := strings.TrimSpace(r.Namespace)
	if namespace != "" {
		return namespace
	}

	if envNamespace := strings.TrimSpace(os.Getenv("POD_NAMESPACE")); envNamespace != "" {
		return envNamespace
	}

	return "default"
}

func (r *K8sJobRunner) resolveCPULimit() string {
	cpuLimit := strings.TrimSpace(r.CPULimit)
	if cpuLimit == "" {
		return "1"
	}

	return cpuLimit
}

func (r *K8sJobRunner) resolvePollInterval() time.Duration {
	if r.PollInterval <= 0 {
		return 350 * time.Millisecond
	}
	if r.PollInterval < 150*time.Millisecond {
		return 150 * time.Millisecond
	}

	return r.PollInterval
}

func (r *K8sJobRunner) resolveJobTTLSeconds() int32 {
	if r.JobTTLSeconds <= 0 {
		return 120
	}

	return r.JobTTLSeconds
}

func (r *K8sJobRunner) resolveKubectlBin() string {
	kubectlBin := strings.TrimSpace(r.KubectlBin)
	if kubectlBin == "" {
		return "kubectl"
	}

	return kubectlBin
}

func (r *K8sJobRunner) runKubectl(input kubectlRunInput) (string, error) {
	command := exec.CommandContext(input.Context, r.resolveKubectlBin(), input.Args...)
	if input.Stdin != "" {
		command.Stdin = strings.NewReader(input.Stdin)
	}

	command.Env = os.Environ()
	kubeconfigPath := strings.TrimSpace(r.KubeconfigPath)
	if kubeconfigPath != "" {
		command.Env = append(command.Env, "KUBECONFIG="+kubeconfigPath)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command.Stdout = stdout
	command.Stderr = stderr

	err := command.Run()
	if err == nil {
		return stdout.String(), nil
	}

	trimmedOutput := strings.TrimSpace(stderr.String())
	if trimmedOutput == "" {
		trimmedOutput = strings.TrimSpace(stdout.String())
	}
	if trimmedOutput == "" {
		return "", err
	}

	return "", fmt.Errorf("%w: %s", err, trimmedOutput)
}
