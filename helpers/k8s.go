package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"regexp"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/models"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	k8sConfigMapRegexValidation = `^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	k8sConfigMapValidationError = "invalid name '%s', should be lowercase alphanumeric characters or '-' and '.'"
)

func ValidateK8sConfigMapName(name string) error {
	// Check if the name matches the regex
	matched, err := regexp.MatchString(k8sConfigMapRegexValidation, name)
	if err != nil {
		return fmt.Errorf("failed to validate object name: %w", err)
	}
	if !matched {
		return fmt.Errorf(k8sConfigMapValidationError, name)
	}
	return nil
}

func ListConfigMaps(kube config.KubeConfig) (*corev1.ConfigMapList, error) {
	configMaps, err := kube.Clientset.CoreV1().ConfigMaps(
		kube.Namespace).List(
		context.TODO(), metav1.ListOptions{})

	if err != nil {
		log.Println(err)
	}

	return configMaps, err
}

func GetConfigMap(kube config.KubeConfig, name string) (*corev1.ConfigMap, error) {
	configMap, err := kube.Clientset.CoreV1().ConfigMaps(
		kube.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		log.Println(err)
		return nil, err
	}

	return configMap, nil
}

func CreateOrUpdateConfigMap(kube config.KubeConfig, data map[string]string, operation string) (*corev1.ConfigMap, error) {
	configMap := ConfigMap(data, kube.Namespace)
	var ret *corev1.ConfigMap
	var err error

	// Operations permetter : "create" and "update"
	if operation == "create" {
		ret, err = kube.Clientset.CoreV1().ConfigMaps(
			kube.Namespace).Create(
			context.TODO(), configMap, metav1.CreateOptions{})
	} else if operation == "update" {
		ret, err = kube.Clientset.CoreV1().ConfigMaps(
			kube.Namespace).Update(
			context.TODO(), configMap, metav1.UpdateOptions{})
	}

	if err != nil {
		return nil, err
	}

	return ret, nil
}

func DeleteConfigMap(kube config.KubeConfig, name string) error {
	err := kube.Clientset.CoreV1().ConfigMaps(
		kube.Namespace).Delete(
		context.TODO(), name, metav1.DeleteOptions{})

	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func ConfigMap(data map[string]string, namespace string) *corev1.ConfigMap {
	name := data["name"]

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}

}

func GetSecret(kube config.KubeConfig, secretName string) (*corev1.Secret, error) {
	secret, err := kube.Clientset.CoreV1().Secrets(
		kube.Namespace).Get(
		context.TODO(), secretName, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	return secret, nil
}

func CreateOrUpdateSecret(kube config.KubeConfig, name string, data map[string]string, operation string) (*corev1.Secret, error) {
	secret := Secret(name, kube.Namespace, data)

	var ret *corev1.Secret
	var err error

	// Operations permetter : "create" and "update"
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
		log.Println(err)
		return nil, err
	}

	return ret, nil
}

func DeleteSecret(kube config.KubeConfig, name string) error {
	err := kube.Clientset.CoreV1().Secrets(
		kube.Namespace).Delete(
		context.TODO(), name, metav1.DeleteOptions{})

	if err != nil {
		return err
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
			log.Println(err)
			return nil, err
		}
	} else {
		for _, labelSelector := range labelSelectors {
			job, err := kube.Clientset.BatchV1().Jobs(
				kube.Namespace).List(
				context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
			if err != nil {
				log.Println(err)
				return nil, err
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

func GetJob(kube config.KubeConfig, name string) (*batchv1.Job, error) {
	job, err := kube.Clientset.BatchV1().Jobs(
		kube.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		log.Println(err)
		return nil, err
	}

	return job, nil
}

// TODO: Too many arguments, will need a rework
func CreateJob(kube config.KubeConfig, name string, runnerName string, runnerImage string, owner string, extraVars string, command string, gitURL string, gitBranch string) (string, error) {
	job := JobObject(name, kube, runnerName, runnerImage, owner, extraVars, command, gitURL, gitBranch)

	job, err := kube.Clientset.BatchV1().Jobs(
		kube.Namespace).Create(
		context.TODO(), job, metav1.CreateOptions{})

	if err != nil {
		log.Println(err)
		return "", err
	}

	return job.Name, nil
}

func ListPods(kube config.KubeConfig, labelSelector string) (*corev1.PodList, error) {
	pods, err := kube.Clientset.CoreV1().Pods(
		kube.Namespace).List(context.TODO(),
		v1.ListOptions{LabelSelector: labelSelector})

	if err != nil {
		log.Println(err)
		return nil, err
	}

	return pods, err
}

// TODO: Need to implement logs for init-containers
func GetLogs(kube config.KubeConfig, podName string, containerName string) (string, error) {
	podLogOpts := corev1.PodLogOptions{
		Container: containerName,
	}

	req := kube.Clientset.CoreV1().Pods(kube.Namespace).GetLogs(podName, &podLogOpts)

	podLogs, err := req.Stream(context.TODO())
	if err != nil {
		return "", err
	}

	defer podLogs.Close()

	buf := new(bytes.Buffer)

	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func JobObject(name string,
	kube config.KubeConfig,
	runnerName string,
	image string, owner string,
	extraVars string,
	command string,
	gitURL string,
	gitBranch string) *batchv1.Job {

	var ttlSeconds = int32(kube.JobsTTL)
	var backoffLimit int32 = 1

	optionalSecret := true

	initCmd := fmt.Sprintf("git clone -b %s %s . ; git ls-remote", gitBranch, gitURL)

	env := []corev1.EnvVar{}
	// Append extra vars to environment variables only if provided
	if extraVars != "" {
		env = append(env, corev1.EnvVar{
			Name:  "EXTRA_VARS",
			Value: extraVars,
		})
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name + "-",
			Namespace:    kube.Namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttlSeconds,
			BackoffLimit:            &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"owner":     owner,
						"task-name": name,
					},
				},
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{
						{
							Name: "secret",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: runnerName,
									Optional:   &optionalSecret,
								},
							},
						},
						{
							Name: "repo",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            name,
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"sh",
								"-c",
								command,
							},
							WorkingDir: "/mnt/repo",
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "secret",
									MountPath: "/etc/secret/",
									ReadOnly:  true,
								},
								{
									Name:      "repo",
									MountPath: "/mnt/repo",
									ReadOnly:  false,
								},
							},
							Env: env,
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: runnerName,
										},
										Optional: &optionalSecret,
									},
								},
							},
						},
					},
					InitContainers: []corev1.Container{
						{
							Name:            "init-" + name,
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
						},
					},
				},
			},
		},
	}
}

func ListCronJobs(kube config.KubeConfig, labelSelectors []string) (*batchv1.CronJobList, error) {
	var jobsList *batchv1.CronJobList
	var err error

	if len(labelSelectors) == 0 {
		jobsList, err = kube.Clientset.BatchV1().CronJobs(
			kube.Namespace).List(
			context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Println(err)
			return nil, err
		}
	} else {
		for _, labelSelector := range labelSelectors {
			job, err := kube.Clientset.BatchV1().CronJobs(
				kube.Namespace).List(
				context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
			if err != nil {
				log.Println(err)
				return nil, err
			}
			if jobsList == nil {
				jobsList = job
			} else {
				jobsList.Items = append(jobsList.Items, job.Items...)
			}
		}
	}

	return jobsList, nil
}

func GetCronJob(kube config.KubeConfig, name string) (*batchv1.CronJob, error) {
	job, err := kube.Clientset.BatchV1().CronJobs(
		kube.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		log.Println(err)
		return nil, err
	}

	return job, nil
}

func CreateOrUpdateCronJob(kube config.KubeConfig, cronjob models.CronJob, runner *corev1.ConfigMap, command string, operation string) (*batchv1.CronJob, error) {
	var extraVars string
	var err error

	if len(cronjob.ExtraVars) > 0 {
		varsParsed, err := json.Marshal(cronjob.ExtraVars)
		if err != nil {
			return nil, err
		}
		extraVars = string(varsParsed)
	}

	jobObj := JobObject(cronjob.Task,
		kube,
		runner.Data["name"],
		runner.Data["image"],
		cronjob.Owner,
		extraVars,
		command,
		runner.Data["gitURL"],
		runner.Data["branch"],
	)
	cron := CronJobObject(kube, cronjob, jobObj.Spec)

	if operation == "create" {
		cron, err = kube.Clientset.BatchV1().CronJobs(
			kube.Namespace).Create(
			context.TODO(), cron, metav1.CreateOptions{})
	} else if operation == "update" {
		cron, err = kube.Clientset.BatchV1().CronJobs(
			kube.Namespace).Update(
			context.TODO(), cron, metav1.UpdateOptions{})
	}

	if err != nil {
		log.Println(err)
		return nil, err
	}

	return cron, nil
}

func DeleteCronJob(kube config.KubeConfig, name string) error {
	err := kube.Clientset.BatchV1().CronJobs(
		kube.Namespace).Delete(
		context.TODO(), name, metav1.DeleteOptions{})

	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func CronJobObject(kube config.KubeConfig, cronjob models.CronJob, jobSpec batchv1.JobSpec) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronjob.Name,
			Namespace: kube.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: cronjob.Schedule,
			Suspend:  &cronjob.Disable,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: jobSpec,
			},
		},
	}
}
