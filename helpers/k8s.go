package helpers

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/kriten-io/kriten/config"

	uuid "github.com/satori/go.uuid"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	k8sObjNameRegexValidation = `^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
)

func ValidateK8sObjName(name string) error {
	// Check if the name matches the regex
	matched, err := regexp.MatchString(k8sObjNameRegexValidation, name)
	if err != nil {
		return fmt.Errorf("failed to validate object name: %w", err)
	}
	if !matched {
		return fmt.Errorf("object name %s does not meet k8s config naming policy", name)
	}
	return nil
}

func ListConfigMaps(kube config.KubeConfig) (*corev1.ConfigMapList, error) {
	configMaps, err := kube.Clientset.CoreV1().ConfigMaps(
		kube.Namespace).List(
		context.TODO(), metav1.ListOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to list configmaps: %w", err)
	}

	return configMaps, nil
}

// func GetConfigMap(kube config.KubeConfig, name string) (*corev1.ConfigMap, error) {
// 	configMap, err := kube.Clientset.CoreV1().ConfigMaps(
// 		kube.Namespace).Get(
// 		context.TODO(), name, metav1.GetOptions{})

// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get configmap %s: %w", name, err)
// 	}

// 	return configMap, nil
// }

// func CreateOrUpdateConfigMap(kube config.KubeConfig, data map[string]string, operation string) (*corev1.ConfigMap, error) {
// 	configMap := ConfigMap(data, kube.Namespace)
// 	var ret *corev1.ConfigMap
// 	var err error

// 	if operation == "create" {
// 		ret, err = kube.Clientset.CoreV1().ConfigMaps(
// 			kube.Namespace).Create(
// 			context.TODO(), configMap, metav1.CreateOptions{})
// 	} else if operation == "update" {
// 		ret, err = kube.Clientset.CoreV1().ConfigMaps(
// 			kube.Namespace).Update(
// 			context.TODO(), configMap, metav1.UpdateOptions{})
// 	}

// 	if err != nil {
// 		return nil, fmt.Errorf("failed to %s configmap: %w", operation, err)
// 	}

// 	return ret, nil
// }

// func DeleteConfigMap(kube config.KubeConfig, name string) error {
// 	err := kube.Clientset.CoreV1().ConfigMaps(
// 		kube.Namespace).Delete(
// 		context.TODO(), name, metav1.DeleteOptions{})

// 	if err != nil {
// 		return fmt.Errorf("failed to delete configmap %s: %w", name, err)
// 	}

// 	return nil
// }

// func ConfigMap(data map[string]string, namespace string) *corev1.ConfigMap {
// 	name := data["name"]

// 	return &corev1.ConfigMap{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name:      name,
// 			Namespace: namespace,
// 		},
// 		Data: data,
// 	}

// }

func GetSecret(kube config.KubeConfig, secretName string) (*corev1.Secret, error) {
	secret, err := kube.Clientset.CoreV1().Secrets(
		kube.Namespace).Get(
		context.TODO(), secretName, metav1.GetOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	return secret, nil
}

func CreateOrUpdateSecret(kube config.KubeConfig, name string, data map[string]string, operation string) (*corev1.Secret, error) {
	secret := Secret(name, kube.Namespace, data)

	var ret *corev1.Secret
	var err error

	if operation == "create" {
		ret, err = kube.Clientset.CoreV1().Secrets(
			kube.Namespace).Create(
			context.TODO(), secret, metav1.CreateOptions{})
	} else if operation == "update" {
		ret, err = kube.Clientset.CoreV1().Secrets(
			kube.Namespace).Update(
			context.TODO(), secret, metav1.UpdateOptions{})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to %s secret: %w", operation, err)
	}

	return ret, nil
}

func DeleteSecret(kube config.KubeConfig, name string) error {
	err := kube.Clientset.CoreV1().Secrets(
		kube.Namespace).Delete(
		context.TODO(), name, metav1.DeleteOptions{})

	if err != nil {
		return fmt.Errorf("failed to delete secret %s: %w", name, err)
	}

	return nil
}

func Secret(name string, namespace string, data map[string]string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		StringData: data,
	}
}

func ListJobs(kube config.KubeConfig, labelSelectors []string) (*batchv1.JobList, error) {
	var jobsList *batchv1.JobList
	var err error

	if len(labelSelectors) == 0 {
		jobsList, err = kube.Clientset.BatchV1().Jobs(
			kube.Namespace).List(
			context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list jobs: %w", err)
		}
	} else {
		for _, labelSelector := range labelSelectors {
			job, err := kube.Clientset.BatchV1().Jobs(
				kube.Namespace).List(
				context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
			if err != nil {
				return nil, fmt.Errorf("failed to list jobs with selector %s: %w", labelSelector, err)
			}
			if jobsList == nil {
				jobsList = job
			} else {
				jobsList.Items = append(jobsList.Items, job.Items[:]...)
			}
		}
	}

	return jobsList, nil
}

func DeleteJob(kube config.KubeConfig, name string) error {
	propagation := metav1.DeletePropagationBackground
	err := kube.Clientset.BatchV1().Jobs(
		kube.Namespace).Delete(
		context.TODO(), name, metav1.DeleteOptions{PropagationPolicy: &propagation})

	if err != nil && !k8sErrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete job %s: %w", name, err)
	}

	return nil
}

func GetJob(kube config.KubeConfig, name string) (*batchv1.Job, error) {
	job, err := kube.Clientset.BatchV1().Jobs(
		kube.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to get job %s: %w", name, err)
	}

	return job, nil
}

// GenerateName returns a DNS-1123 compatible name with a random suffix, used
// for ephemeral per-job secrets.
func GenerateName(prefix string) (string, error) {
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("failed to generate random name: %w", err)
	}

	return fmt.Sprintf("%s-%s", prefix, hex.EncodeToString(suffix)), nil
}

// TODO: Too many arguments, will need a rework
func CreateJob(kube config.KubeConfig, taskID uuid.UUID, taskName string, envSecret string, tokenSecret string, runnerImage string, owner string,
	extraVars string, command string, gitURL string, gitBranch string) (*batchv1.Job, error) {
	job := JobObject(taskID, taskName, kube, envSecret, tokenSecret, runnerImage, owner, extraVars, command, gitURL, gitBranch)

	created, err := kube.Clientset.BatchV1().Jobs(
		kube.Namespace).Create(
		context.TODO(), job, metav1.CreateOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	return created, nil
}

// SetSecretOwnerReference makes the given job the owner of the secret so that
// Kubernetes garbage collects the secret when the job (and its TTL) is removed.
func SetSecretOwnerReference(kube config.KubeConfig, secretName string, job *batchv1.Job) error {
	secret, err := kube.Clientset.CoreV1().Secrets(
		kube.Namespace).Get(
		context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get secret %s: %w", secretName, err)
	}

	controller := true
	secret.OwnerReferences = append(secret.OwnerReferences, metav1.OwnerReference{
		APIVersion: "batch/v1",
		Kind:       "Job",
		Name:       job.Name,
		UID:        job.UID,
		Controller: &controller,
	})

	if _, err := kube.Clientset.CoreV1().Secrets(
		kube.Namespace).Update(
		context.TODO(), secret, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to set owner reference on secret %s: %w", secretName, err)
	}

	return nil
}

func ListPods(kube config.KubeConfig, labelSelector string) (*corev1.PodList, error) {
	pods, err := kube.Clientset.CoreV1().Pods(
		kube.Namespace).List(context.TODO(),
		v1.ListOptions{LabelSelector: labelSelector})

	if err != nil {
		return nil, fmt.Errorf("failed to list pods with selector %s: %w", labelSelector, err)
	}

	return pods, nil
}

// TODO: Need to implement logs for init-containers
func GetLogs(kube config.KubeConfig, podName string, containerName string) (string, error) {
	podLogOpts := corev1.PodLogOptions{
		Container: containerName,
	}

	req := kube.Clientset.CoreV1().Pods(kube.Namespace).GetLogs(podName, &podLogOpts)

	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to get logs for pod %s container %s: %w", podName, containerName, err)
	}

	defer podLogs.Close()

	buf := new(bytes.Buffer)

	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("failed to read logs for pod %s: %w", podName, err)
	}

	return buf.String(), nil
}

func JobObject(taskID uuid.UUID,
	taskName string,
	kube config.KubeConfig,
	envSecret string,
	tokenSecret string,
	image string, owner string,
	extraVars string,
	command string,
	gitURL string,
	gitBranch string) *batchv1.Job {
	var job *batchv1.Job
	var ttlSeconds = int32(kube.JobsTTL)
	var backoffLimit int32 = 1

	optionalSecret := true

	// The git token is provided through the token secret as an environment
	// variable so that it never appears in the pod spec.
	initCmd := `if [ -n "$GIT_TOKEN" ]; then REPO_URL=$(printf '%s' "$GIT_URL" | sed "s#://#://$GIT_TOKEN@#"); else REPO_URL="$GIT_URL"; fi; git clone -b "$GIT_BRANCH" "$REPO_URL" . ; git ls-remote -q`

	env := []corev1.EnvVar{}
	// Append extra vars to environment variables only if provided
	if extraVars != "" {
		env = append(env, corev1.EnvVar{
			Name:  "EXTRA_VARS",
			Value: extraVars,
		})
	}

	volumes := []corev1.Volume{
		{
			Name: "repo",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}

	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "repo",
			MountPath: "/mnt/repo",
			ReadOnly:  false,
		},
	}

	envFrom := []corev1.EnvFromSource{}

	// Custom runner secrets are injected as a batch through the per-job secret.
	if envSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: envSecret,
					Optional:   &optionalSecret,
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "secret",
			MountPath: "/etc/secret/",
			ReadOnly:  true,
		})
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: envSecret,
				},
				Optional: &optionalSecret,
			},
		})
	}

	jobNamePrefix := strings.ReplaceAll(taskName, "_", "-")
	if jobNamePrefix == "" {
		jobNamePrefix = taskID.String()
	}
	job = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: jobNamePrefix + "-",
			Namespace:    kube.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttlSeconds,
			BackoffLimit:            &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"owner":   owner,
						"task-id": taskID.String(),
					},
				},
				Spec: corev1.PodSpec{
					Volumes:       volumes,
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            taskName,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"sh",
								"-c",
								command,
							},
							WorkingDir:   "/mnt/repo",
							VolumeMounts: volumeMounts,
							Env:          env,
							EnvFrom:      envFrom,
						},
					},
				},
			},
		},
	}

	if gitURL != "" {
		initEnv := []corev1.EnvVar{
			{
				Name:  "GIT_URL",
				Value: gitURL,
			},
			{
				Name:  "GIT_BRANCH",
				Value: gitBranch,
			},
		}

		if tokenSecret != "" {
			initEnv = append(initEnv, corev1.EnvVar{
				Name: "GIT_TOKEN",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: tokenSecret,
						},
						Key: "GIT_TOKEN",
					},
				},
			})
		}

		job.Spec.Template.Spec.InitContainers = []corev1.Container{
			{
				Name:            "init-" + taskName,
				Image:           "bitnami/git",
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command: []string{
					"sh",
					"-c",
					initCmd,
				},
				WorkingDir: "/mnt/repo",
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "repo",
						MountPath: "/mnt/repo",
					},
				},
				Env: initEnv,
			},
		}
	}

	return job
}
