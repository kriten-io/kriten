package main

import (
	"fmt"
	"kriten-core/config"
	"kriten-core/controllers"
	"kriten-core/helpers"
	"kriten-core/services"
	"log"
	"path/filepath"

	docs "kriten-core/docs"

	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/elastic/go-elasticsearch/v8"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	router     *gin.Engine
	as         services.AuthService
	rs         services.RunnerService
	ts         services.TaskService
	js         services.JobService
	us         services.UserService
	rls        services.RoleService
	rbs        services.RoleBindingService
	ac         controllers.AuthController
	rc         controllers.RunnerController
	tc         controllers.TaskController
	jc         controllers.JobController
	uc         controllers.UserController
	rlc        controllers.RoleController
	rbc        controllers.RoleBindingController
	conf       config.Config
	kubeConfig *rest.Config
	es         helpers.ElasticSearch
	db         *gorm.DB

	GitBranch string
)

var authProviders = []string{"local"}

func init() {
	// Loading env variables and creating the config
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	conf = config.NewConfig(GitBranch)

	// Retrieving k8s clientset
	if conf.Environment == "production" {
		// creates the in-cluster config
		kubeConfig, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	} else {
		// Kubeconfig file will be fetched from the home folder for development purpose.
		home := homedir.HomeDir()
		configPath := filepath.Join(home, ".kube", "config")
		log.Printf("Using local kube config path: %s\n", configPath)
		kubeConfig, err = clientcmd.BuildConfigFromFlags("", configPath)
		if err != nil {
			panic(err.Error())
		}
	}
	conf.Kube.Clientset, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		panic(err.Error())
	}

	// Establishing connection with PostgreSQL database
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%v sslmode=%s",
		conf.DB.Host,
		conf.DB.User,
		conf.DB.Password,
		conf.DB.Name,
		conf.DB.Port,
		conf.DB.SSL,
	)
	db, err = gorm.Open(postgres.New(postgres.Config{
		DSN:                  dsn,
		PreferSimpleProtocol: true, // disables implicit prepared statement usage
	}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Println("Error while connecting to Postgres")
		log.Println(err)
	}
	config.InitDB(db)

	if conf.ElasticSearch.CloudID != "" {
		es.Client, err = elasticsearch.NewClient(
			elasticsearch.Config{
				CloudID: conf.ElasticSearch.CloudID,
				APIKey:  conf.ElasticSearch.APIKey,
			})
		es.Index = conf.ElasticSearch.Index

		if err != nil {
			log.Println("Error while connecting to ElasticSearch")
			log.Println(err)
		} else {
			es.Enabled = true
		}
	}
}

func init() {
	// Services
	us = services.NewUserService(db, conf)
	rls = services.NewRoleService(db, conf, &rbs, &us)
	rbs = services.NewRoleBindingService(db, conf, rls, us)
	as = services.NewAuthService(conf, us, rls, rbs)

	rs = services.NewRunnerService(conf)
	ts = services.NewTaskService(conf)
	js = services.NewJobService(conf)

	// only enabling active directory if not a community release
	if !conf.CommunityRelease {
		authProviders = append(authProviders, "active_directory")
	}

	// Controllers
	uc = controllers.NewUserController(us, as, es, authProviders)
	rlc = controllers.NewRoleController(rls, as, es)
	rbc = controllers.NewRoleBindingController(rbs, as, es, authProviders)
	ac = controllers.NewAuthController(as, es, authProviders)

	rc = controllers.NewRunnerController(rs, as, es)
	tc = controllers.NewTaskController(ts, as, es)
	jc = controllers.NewJobController(js, as, es)
}

//	@title			Swagger Kriten
//	@version		v0.3
//	@description	API Gateway for your kubernetes services.
//	@termsOfService	http://swagger.io/terms/

//	@contact.name	Evolvere Support
//	@contact.url	https://www.evolvere-tech.co.uk/contact
//	@contact.email	info@evolvere-tech.co.uk

//	@license.name	Apache 2.0
//	@license.url	http://www.apache.org/licenses/LICENSE-2.0.html

//	@BasePath	/api/v1

//	@securityDefinitions.apikey	Bearer
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer" followed by a space and JWT token.

func main() {
	// API endpoints definition, fields starting with ':' are not fixed and can contain any string
	// Expected path: /api/v1/runner/:rname/task/:tname
	router = gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Content-Range", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	docs.SwaggerInfo.BasePath = "/api/v1"
	basepath := router.Group("/api/v1")
	{
		ac.SetAuthRoutes(basepath)
		runners := basepath.Group("/runners")
		tasks := basepath.Group("/tasks")
		jobs := basepath.Group("/jobs")
		users := basepath.Group("/users")
		roles := basepath.Group("/roles")
		roleBindings := basepath.Group("/role_bindings")
		{
			rc.SetRunnerRoutes(runners, conf)
			tc.SetTaskRoutes(tasks, conf)
			jc.SetJobRoutes(jobs, conf)
			uc.SetUserRoutes(users, conf)
			rlc.SetRoleRoutes(roles, conf)
			rbc.SetRoleBindingRoutes(roleBindings, conf)
		}
	}

	log.Fatal(router.Run())
}
