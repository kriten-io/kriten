package config

import (
	"log"

	"github.com/kriten-io/kriten/models"
	"github.com/lib/pq"

	"golang.org/x/crypto/bcrypt"

	"gorm.io/gorm"
)

func InitDB(db *gorm.DB, rp string) {
	err := db.AutoMigrate(
		&models.AuditLog{},
		&models.Group{},
		&models.Role{},
		&models.User{},
		&models.ApiToken{},
		&models.Webhook{},
	)
	if err != nil {
		log.Println("Error during Postgres AutoMigrate")
		log.Println(err)
	}

	bytes, err := bcrypt.GenerateFromPassword([]byte(rp), bcrypt.DefaultCost)
	if err != nil {
		return
	}
	root_passwd_hashed := string(bytes)

	var root = models.User{Username: "root", Password: root_passwd_hashed, Provider: "local", Builtin: true}
	db.FirstOrCreate(&root)

	if err := db.Where(&root).
		Assign(&root).
		FirstOrCreate(&models.User{}).Error; err != nil {
		return
	}

	var adminRole = models.Role{
		Name: "Admin", Resource: "*",
		Resource_Names: pq.StringArray{"*"},
		Access:         "write",
		Builtin:        true,
	}
	db.FirstOrCreate(&adminRole)

	var adminGroup = models.Group{
		Name:     "Admin",
		Provider: "local",
		User_IDs: pq.StringArray{root.ID.String()},
		Role_IDs: pq.StringArray{adminRole.ID.String()},
		Builtin:  true,
	}
	db.FirstOrCreate(&adminGroup)

	db.Updates(&root)

	var builtinRoles = []models.Role{
		{Name: "WriteAllRunners", Resource: "runners", Resource_Names: pq.StringArray{"*"}, Access: "write", Builtin: true},
		{Name: "WriteAllTasks", Resource: "tasks", Resource_Names: pq.StringArray{"*"}, Access: "write", Builtin: true},
		{Name: "ExecuteAllTasks", Resource: "tasks", Resource_Names: pq.StringArray{"*"}, Access: "execute", Builtin: true},
		{Name: "WriteAllUsers", Resource: "users", Resource_Names: pq.StringArray{"*"}, Access: "write", Builtin: true},
		{Name: "WriteAllRoles", Resource: "roles", Resource_Names: pq.StringArray{"*"}, Access: "write", Builtin: true},
	}
	db.Create(&builtinRoles)

	// rules to preveng builtin deletion or update
	db.Exec("CREATE RULE builtin_del_users AS ON DELETE TO users WHERE builtin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_del_groups AS ON DELETE TO groups WHERE builtin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_del_roles AS ON DELETE TO roles WHERE builtin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_upd_roles AS ON UPDATE TO roles WHERE old.builtin DO INSTEAD nothing;")
}
