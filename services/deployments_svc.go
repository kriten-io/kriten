package services

import (
	"fmt"
	"kriten/config"
	"kriten/helpers"
	"kriten/models"

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
	var jobsList []models.Deployment

	return jobsList, nil
}

func (d *DeploymentServiceImpl) GetDeployment(name string) (models.Deployment, error) {
	var deploy models.Deployment

	return deploy, nil
}

func (d *DeploymentServiceImpl) CreateDeployment(deploy models.Deployment) (models.Deployment, error) {
	return deploy, nil
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
