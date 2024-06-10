package services

import (
	"fmt"
	"kriten/config"
	"kriten/helpers"
	"kriten/models"
	"log"

	"encoding/json"
)

type DeploymentService interface {
	ListDeployments([]string) ([]models.Deployment, error)
	GetDeployment(string) (models.Deployment, error)
	CreateDeployment(models.Deployment) (models.Deployment, error)
	UpdateDeployment(models.Deployment) (models.Deployment, error)
	DeleteDeployment(string) error
	GetSchema(string) (map[string]interface{}, error)
}

type DeploymentServiceImpl struct {
	config config.Config
}

func NewDeploymentService(config config.Config) DeploymentService {
	return &DeploymentServiceImpl{
		config: config,
	}
}

func (d *DeploymentServiceImpl) ListDeployments(authList []string) ([]models.Deployment, error) {
	var deployments []models.Deployment
	var labelSelector []string

	if len(authList) == 0 {
		return deployments, nil
	}

	if authList[0] != "*" {
		for _, s := range authList {
			labelSelector = append(labelSelector, "task-name="+s)
		}
	}

	deploysList, err := helpers.ListDeployments(d.config.Kube, labelSelector)
	if err != nil {
		return nil, err
	}

	for _, d := range deploysList.Items {
		// TODO: this is just a temporary fix, will need a more robust solution
		if d.Name == "kriten" || d.Name == "kriten-frontend" {
			continue
		}
		var data map[string]interface{}
		// This unmarshal is only used to fetch the extra vars, it doesn't look very reliable so it might need a rework
		containerEnv := d.Spec.Template.Spec.Containers[0].Env
		log.Println(containerEnv)
		if len(containerEnv) > 0 {
			err = json.Unmarshal([]byte(containerEnv[0].Value), &data)
			if err != nil {
				return nil, err
			}
		}
		dRet := models.Deployment{
			Name:      d.Name,
			Owner:     d.Spec.Template.Labels["owner"],
			Task:      d.Spec.Template.Labels["task-name"],
			Replicas:  *d.Spec.Replicas,
			ExtraVars: data,
		}
		deployments = append(deployments, dRet)
	}

	return deployments, nil
}

func (d *DeploymentServiceImpl) GetDeployment(name string) (models.Deployment, error) {
	var deploy models.Deployment

	return deploy, nil
}

func (d *DeploymentServiceImpl) CreateDeployment(deploy models.Deployment) (models.Deployment, error) {
	runner, command, err := PreFlightChecks(d.config.Kube, deploy.Task, deploy.ExtraVars)
	if err != nil {
		return models.Deployment{}, err
	}

	_, err = helpers.CreateOrUpdateDeployment(d.config.Kube, deploy, runner, command, "create")

	return deploy, err
}

func (d *DeploymentServiceImpl) UpdateDeployment(deploy models.Deployment) (models.Deployment, error) {
	return deploy, nil
}

func (d *DeploymentServiceImpl) DeleteDeployment(id string) error {
	err := helpers.DeleteDeployment(d.config.Kube, id)
	if err != nil {
		return err
	}

	return nil
}

func (d *DeploymentServiceImpl) GetSchema(name string) (map[string]interface{}, error) {
	var data map[string]interface{}

	configMap, err := helpers.GetConfigMap(d.config.Kube, name)
	if err != nil {
		return nil, err
	}
	if configMap.Data["runner"] == "" {
		return nil, fmt.Errorf("task %s not found", name)
	}

	if configMap.Data["schema"] != "" {
		err = json.Unmarshal([]byte(configMap.Data["schema"]), &data)
		if err != nil {
			return nil, err
		}

	}

	return data, nil
}
