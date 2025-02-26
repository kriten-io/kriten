package main

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/kriten-io/kriten/config"
	"github.com/kriten-io/kriten/controllers"
	"github.com/kriten-io/kriten/services"

	docs "github.com/kriten-io/kriten/docs"

	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

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
	cjs        services.CronJobService
	us         services.UserService
	ats        services.ApiTokenService
	gs         services.GroupService
	als        services.AuditService
	rls        services.RoleService
	rbs        services.RoleBindingService
	ac         controllers.AuthController
	alc        controllers.AuditController
	rc         controllers.RunnerController
	tc         controllers.TaskController
	jc         controllers.JobController
	cjc        controllers.CronJobController
	uc         controllers.UserController
	atc        controllers.ApiTokenController
	gc         controllers.GroupController
	rlc        controllers.RoleController
	rbc        controllers.RoleBindingController
	conf       config.Config
	kubeConfig *rest.Config
	// es         helpers.ElasticSearch
	db *gorm.DB

	GitBranch string
)

var authProviders = []string{"local", "active_directory"}

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

	connected := false
	for !connected {
		db, err = gorm.Open(postgres.New(postgres.Config{
			DSN:                  dsn,
			PreferSimpleProtocol: true, // disables implicit prepared statement usage
		}), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			log.Println("Error while connecting to Postgres")
			log.Println(err)
			log.Println("Retrying in 30 seconds..")
			time.Sleep(30 * time.Second)
		} else {
			connected = true
		}
	}
	config.InitDB(db)

	// if conf.ElasticSearch.CloudID != "" {
	// 	es.Client, err = elasticsearch.NewClient(
	// 		elasticsearch.Config{
	// 			CloudID: conf.ElasticSearch.CloudID,
	// 			APIKey:  conf.ElasticSearch.APIKey,
	// 		})
	// 	es.Index = conf.ElasticSearch.Index
	//
	// 	if err != nil {
	// 		log.Println("Error while connecting to ElasticSearch")
	// 		log.Println(err)
	// 	} else {
	// 		es.Enabled = true
	// 	}
	// }
}

func init() {
	// Services
	us = services.NewUserService(db, conf)
	ats = services.NewApiTokenService(db, conf)
	gs = services.NewGroupService(db, us, conf)
	rls = services.NewRoleService(db, conf, &rbs, &us)
	rbs = services.NewRoleBindingService(db, conf, rls, gs)
	as = services.NewAuthService(conf, us, rls, rbs, db)
	als = services.NewAuditService(db, conf)

	rs = services.NewRunnerService(conf)
	ts = services.NewTaskService(conf)
	js = services.NewJobService(conf)
	cjs = services.NewCronJobService(conf)

	// Controllers
	uc = controllers.NewUserController(us, as, als, authProviders)
	atc = controllers.NewApiTokenController(ats, as, als, authProviders)
	gc = controllers.NewGroupController(gs, as, als, authProviders)
	rlc = controllers.NewRoleController(rls, as, als)
	rbc = controllers.NewRoleBindingController(rbs, as, als, authProviders)
	ac = controllers.NewAuthController(as, als, authProviders)
	alc = controllers.NewAuditController(als, as)

	rc = controllers.NewRunnerController(rs, as, als)
	tc = controllers.NewTaskController(ts, as, als)
	jc = controllers.NewJobController(js, as, als)
	cjc = controllers.NewCronJobController(cjs, as, als)
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
		audit := basepath.Group("/audit_logs")
		runners := basepath.Group("/runners")
		tasks := basepath.Group("/tasks")
		jobs := basepath.Group("/jobs")
		cronjobs := basepath.Group("/cronjobs")
		users := basepath.Group("/users")
		tokens := basepath.Group("/api_tokens")
		groups := basepath.Group("/groups")
		roles := basepath.Group("/roles")
		roleBindings := basepath.Group("/role_bindings")
		{
			alc.SetAuditRoutes(audit, conf)
			rc.SetRunnerRoutes(runners, conf)
			tc.SetTaskRoutes(tasks, conf)
			jc.SetJobRoutes(jobs, conf)
			cjc.SetCronJobRoutes(cronjobs, conf)
			uc.SetUserRoutes(users, conf)
			atc.SetApiTokenRoutes(tokens, conf)
			gc.SetGroupRoutes(groups, conf)
			rlc.SetRoleRoutes(roles, conf)
			rbc.SetRoleBindingRoutes(roleBindings, conf)
		}
	}

	log.Fatal(router.Run())
}
