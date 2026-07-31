package services

import (
	"errors"
)

var (
	ErrSvcObjInUse              = errors.New("object is referenced by another object")
	ErrSvcObjNameK8sConfMap     = errors.New("object name should contain only lowercase alphanumeric characters, including '-' or '.'")
	ErrSvcObjNotFound           = errors.New("object not found")
	ErrSvcObjExists             = errors.New("object already exists")
	ErrSvcTaskSchemaValidation  = errors.New("schema validation error")
	ErrSvcTaskNoRunner          = errors.New("runner not found")
	ErrSvcCronJobPrecheck       = errors.New("cronjob precheck error")
	ErrSvcAuthProviderMismatch  = errors.New("auth providers mismatch")
	ErrSvcUserGroupMembership   = errors.New("user is a member of group")
	ErrSvcRoleValidation        = errors.New("role validation error")
	ErrSvcRoleBindingValidation = errors.New("role binding validation error")
	ErrSvcDBDuplicatedKey       = errors.New("object name clashes with existing object")
	ErrSvcDeleteBuiltin         = errors.New("cannot delete builtin objects")
)
