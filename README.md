[![Version Release](https://github.com/evolvere-tech/kriten-core/actions/workflows/version-release.yml/badge.svg)](https://github.com/evolvere-tech/kriten-core/actions/workflows/version-release.yml)

# Kriten

## Main features

### Auth flows
 - Authentication via Active Directory credentials
 - Authorisation via Active Directory users' groups

 - K8s secret to define Operators

### Config
 - API endpoint for creating Runners
    - Specify which AD group has access to creating Tasks for that runner

 - Possibility to create an API endpoint that will define a Task
    - Endpoint for updating / modifying the task or access groups

### Runners
 - Python code execution
    - Agree on Python version and imported packages
    - Define example code that should be able to run

 - Ansible tower
    - Possibility to launch pre-defined jobs
    - Store ID of the jobs executed

### Tasks
 - Possibility for authorised users (Consumers) to launch pre-defined Tasks

 - Store information about past jobs (kriten-collector cronjob)
    - Define what do we need to store / How often

 - API endpoint for querying informatio about specific task (ID)

 - API endpoint for retrieving IDs of past jobs

 - API endpoint for querying all past jobs

## API Generated

All endpoints are currently generated under `/api/v1` root

### Shared

#### Endpoints:
    - POST                          /login
    - GET                           /refresh

### Runners
Define a runner image and github repository to fetch your scripts from.

#### Endpoints:
    - GET                           /runners                        // List Runners
    - GET                           /runners/:rID                   // Get Runner
    - [POST,PUT]                    /runners                        // Create Runner
    - [PATCH, PUT]                  /runners/:rID                   // Update Runner
    - DELETE                        /runners/:rID                   // Delete Runner

### Tasks
Define which script you want to make accessible via API

 #### Endpoints:
    - GET                           /tasks                          // List Tasks
    - GET                           /tasks/:tID                     // Get Task
    - [POST,PUT]                    /tasks                          // Create Task
    - [PATCH, PUT]                  /tasks/:tID                     // Update Task
    - DELETE                        /tasks/:tID                     // Delete Task

### Jobs
Execute a script and retrieve output

 #### Endpoints:
    - GET                           /tasks/:tID/jobs                // List Jobs
    - GET                           /tasks/:tID/jobs/:jID           // Get Job
    - [POST,PUT]                    /tasks/:tID/jobs                // Create Tasks
