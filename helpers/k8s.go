package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"kriten/config"
	"kriten/models"
	"log"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
		log.Println(err)
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
		log.Println(err)
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
		if errors.IsNotFound(err) {
			return nil
		}
		log.Println(err)
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

func CreateJob(kube config.KubeConfig, podSpec *corev1.PodSpec, name string, owner string) (string, error) {
	job := JobObject(kube, podSpec, name, owner)

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
		metav1.ListOptions{LabelSelector: labelSelector})

	if err != nil {
		log.Println(err)
		return nil, err
	}

	return pods, err
}

// TODO: Need to implement logs for init-containers
func GetLogs(kube config.KubeConfig, podName string) (string, error) {
	podLogOpts := corev1.PodLogOptions{}

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

func JobObject(kube config.KubeConfig, podSpec *corev1.PodSpec, name string, owner string) *batchv1.Job {
	var ttlSeconds int32 = int32(kube.JobsTTL)
	var backoffLimit int32 = 1

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
				Spec: *podSpec,
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
				jobsList.Items = append(jobsList.Items, job.Items[:]...)
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

	podSpec := PodSpec(cronjob.Task,
		runner.Data["image"],
		extraVars,
		command,
		runner.Data["gitURL"],
		runner.Data["branch"],
	)
	jobObj := JobObject(kube, podSpec, cronjob.Task, cronjob.Owner)
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

func ListDeployments(kube config.KubeConfig, labelSelectors []string) (*appsv1.DeploymentList, error) {
	var deploysList *appsv1.DeploymentList
	var err error

	if len(labelSelectors) == 0 {
		deploysList, err = kube.Clientset.AppsV1().Deployments(
			kube.Namespace).List(
			context.TODO(), metav1.ListOptions{})
		if err != nil {
			log.Println(err)
			return nil, err
		}
	} else {
		for _, labelSelector := range labelSelectors {
			deploy, err := kube.Clientset.AppsV1().Deployments(
				kube.Namespace).List(
				context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
			if err != nil {
				log.Println(err)
				return nil, err
			}
			if deploysList == nil {
				deploysList = deploy
			} else {
				deploysList.Items = append(deploysList.Items, deploy.Items[:]...)
			}
		}

	}

	return deploysList, nil
}

func GetDeployment(kube config.KubeConfig, name string) (*appsv1.Deployment, error) {
	job, err := kube.Clientset.AppsV1().Deployments(
		kube.Namespace).Get(
		context.TODO(), name, metav1.GetOptions{})

	if err != nil {
		log.Println(err)
		return nil, err
	}

	return job, nil
}

func CreateOrUpdateDeployment(kube config.KubeConfig, deploy models.Deployment, runner *corev1.ConfigMap, command string, operation string) (*appsv1.Deployment, error) {
	var extraVars string
	var err error

	if len(deploy.ExtraVars) > 0 {
		varsParsed, err := json.Marshal(deploy.ExtraVars)
		if err != nil {
			return nil, err
		}
		extraVars = string(varsParsed)
	}

	pod := PodSpec(deploy.Task,
		runner.Data["image"],
		extraVars,
		command,
		runner.Data["gitURL"],
		runner.Data["branch"],
	)
	deployment := DeploymentObject(kube, deploy, *pod)

	if operation == "create" {
		deployment, err = kube.Clientset.AppsV1().Deployments(
			kube.Namespace).Create(
			context.TODO(), deployment, metav1.CreateOptions{})
	} else if operation == "update" {
		deployment, err = kube.Clientset.AppsV1().Deployments(
			kube.Namespace).Update(
			context.TODO(), deployment, metav1.UpdateOptions{})
	}

	if err != nil {
		log.Println(err)
		return nil, err
	}

	return deployment, nil
}

func DeleteDeployment(kube config.KubeConfig, name string) error {
	err := kube.Clientset.AppsV1().Deployments(
		kube.Namespace).Delete(
		context.TODO(), name, metav1.DeleteOptions{})

	if err != nil {
		log.Println(err)
		return err
	}

	return nil
}

func DeploymentObject(kube config.KubeConfig, deploy models.Deployment, podSpec corev1.PodSpec) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploy.Name,
			Namespace: kube.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &deploy.Replicas,
			Template: corev1.PodTemplateSpec{
				Spec: podSpec,
			},
		},
	}
}

func PodSpec(name string, image string, extraVars string, command string, gitURL string, gitBranch string) *corev1.PodSpec {
	optional_secret := true

	env := []corev1.EnvVar{}
	// Append extra vars to environment variables only if provided
	if extraVars != "" {
		env = append(env, corev1.EnvVar{
			Name:  "EXTRA_VARS",
			Value: extraVars,
		})
	}

	return &corev1.PodSpec{
		Volumes: []corev1.Volume{
			{
				Name: "secret",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: name,
						Optional:   &optional_secret,
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
								Name: name,
							},
							Optional: &optional_secret,
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
					"git",
				},
				Args: []string{
					"clone",
					"-b",
					gitBranch,
					gitURL,
					".",
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
	}
}
