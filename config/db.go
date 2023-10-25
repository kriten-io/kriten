package config

import (
	"kriten-core/models"
	"log"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

func InitDB(db *gorm.DB) {
	err := db.AutoMigrate(&models.User{}, &models.Role{}, &models.RoleBinding{})
	if err != nil {
		log.Println("Error during Postgres AutoMigrate")
		log.Println(err)
	}

	var root = models.User{Username: "root", Provider: "local", Buitin: true}
	db.FirstOrCreate(&root)

	if err := db.Where(&root).
		Assign(&root).
		FirstOrCreate(&models.User{}).Error; err != nil {
		return
	}

	var adminRole = models.Role{
		Name: "Admin", Resource: "*", Resources_IDs: pq.StringArray{"*"}, Access: "write", Buitin: true,
	}
	db.FirstOrCreate(&adminRole)

	var adminRoleBindings = models.RoleBinding{
		Name: "RootAdminAccess", RoleID: adminRole.ID, RoleName: "Admin", SubjectID: root.ID, SubjectName: "root", SubjectKind: "root", SubjectProvider: "local", Buitin: true,
	}
	db.Create(&adminRoleBindings)

	var builtinRoles = []models.Role{
		{Name: "WriteAllRunners", Resource: "runners", Resources_IDs: pq.StringArray{"*"}, Access: "write", Buitin: true},
		{Name: "WriteAllTasks", Resource: "tasks", Resources_IDs: pq.StringArray{"*"}, Access: "write", Buitin: true},
		{Name: "WriteAllJobs", Resource: "jobs", Resources_IDs: pq.StringArray{"*"}, Access: "write", Buitin: true},
		{Name: "WriteAllUsers", Resource: "users", Resources_IDs: pq.StringArray{"*"}, Access: "write", Buitin: true},
		{Name: "WriteAllRoles", Resource: "roles", Resources_IDs: pq.StringArray{"*"}, Access: "write", Buitin: true},
		{Name: "WriteAllRoleBindings", Resource: "role_bindings", Resources_IDs: pq.StringArray{"*"}, Access: "write", Buitin: true},
	}
	db.Create(&builtinRoles)

	// rules to preveng builtin deletion or update
	db.Exec("CREATE RULE builtin_del_users AS ON DELETE TO users WHERE buitin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_upd_users AS ON UPDATE TO users WHERE old.buitin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_del_roles AS ON DELETE TO roles WHERE buitin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_upd_roles AS ON UPDATE TO roles WHERE old.buitin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_del_rolebindings AS ON DELETE TO role_bindings WHERE buitin DO INSTEAD nothing;")
	db.Exec("CREATE RULE builtin_upd_rolebindings AS ON UPDATE TO role_bindings WHERE old.buitin DO INSTEAD nothing;")
}
