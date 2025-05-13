package config

import (
	"log"

	"github.com/kriten-io/kriten/models"
	"github.com/lib/pq"

	"gorm.io/gorm"
)

func InitDB(db *gorm.DB) {
	err := db.AutoMigrate(
		&models.AuditLog{},
		&models.Group{},
		&models.Role{},
		&models.RoleBinding{},
		&models.User{},
		&models.ApiToken{},
		&models.Webhook{},
	)
	if err != nil {
		log.Println("Error during Postgres AutoMigrate")
		log.Println(err)
	}

	var root = models.User{Username: "root", Provider: "local", Builtin: true, Groups: pq.StringArray{}}
	db.FirstOrCreate(&root)

	if err := db.Where(&root).
		Assign(&root).
		FirstOrCreate(&models.User{}).Error; err != nil {
		return
	}

	var adminRole = models.Role{
		Name: "Admin", Resource: "*", Resources_IDs: pq.StringArray{"*"}, Access: "write", Builtin: true,
	}
	db.FirstOrCreate(&adminRole)

	var adminGroup = models.Group{
		Name: "Admin", Provider: "local", Users: pq.StringArray{root.ID.String()}, Builtin: true,
	}
	db.FirstOrCreate(&adminGroup)

	root.Groups = pq.StringArray{adminGroup.ID.String()}
	db.Updates(&root)

	var adminRoleBindings = models.RoleBinding{
		Name:            "RootAdminAccess",
		RoleID:          adminRole.ID,
		RoleName:        "Admin",
		SubjectID:       adminGroup.ID,
		SubjectName:     "root",
		SubjectKind:     "root",
		SubjectProvider: "local",
		Builtin:         true,
	}
	db.Create(&adminRoleBindings)

	var builtinRoles = []models.Role{
		{Name: "WriteAllRunners", Resource: "runners", Resources_IDs: pq.StringArray{"*"}, Access: "write", Builtin: true},
		{Name: "WriteAllTasks", Resource: "tasks", Resources_IDs: pq.StringArray{"*"}, Access: "write", Builtin: true},
		{Name: "WriteAllJobs", Resource: "jobs", Resources_IDs: pq.StringArray{"*"}, Access: "write", Builtin: true},
		{Name: "WriteAllUsers", Resource: "users", Resources_IDs: pq.StringArray{"*"}, Access: "write", Builtin: true},
		{Name: "WriteAllRoles", Resource: "roles", Resources_IDs: pq.StringArray{"*"}, Access: "write", Builtin: true},
		{Name: "WriteAllRoleBindings", Resource: "role_bindings", Resources_IDs: pq.StringArray{"*"}, Access: "write", Builtin: true},
	}
	db.Create(&builtinRoles)

	// rules to preveng builtin deletion or update
	db.Exec("CREATE RULE builtin_del_users AS ON DELETE TO users WHERE builtin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_upd_users AS ON UPDATE TO users WHERE old.builtin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_del_groups AS ON DELETE TO groups WHERE builtin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_upd_groups AS ON UPDATE TO groups WHERE old.builtin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_del_roles AS ON DELETE TO roles WHERE builtin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_upd_roles AS ON UPDATE TO roles WHERE old.builtin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_del_rolebindings AS ON DELETE TO role_bindings WHERE builtin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_upd_rolebindings AS ON UPDATE TO role_bindings WHERE old.builtin DO INSTEAD nothing;")
}
