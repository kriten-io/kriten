package helpers

import (
	"fmt"
	"kriten/config"
	"log"
	"strings"

	"github.com/go-errors/errors"
	"github.com/go-ldap/ldap/v3"
)

const Filter = "(&(objectClass=organizationalPerson)(sAMAccountName=%s))"

// Ldap Connection without TLS
func ConnectLDAP(config config.LDAPConfig) (*ldap.Conn, error) {
	l, err := ldap.DialURL(fmt.Sprintf("ldap://%s:%d", config.FQDN, config.Port))
	if err != nil {
		log.Println("LDAP connection error: ", err)
		return nil, err
	}

	err = l.Bind(config.BindUser, config.BindPass)

	if err != nil {
		log.Println("Error during readonly Bind", err)
		return nil, err
	}

	return l, nil
}

// Normal Bind and Search
func BindAndSearch(config config.LDAPConfig, user string, password string) error {
	l, err := ConnectLDAP(config)

	if err != nil {
		return err
	}
	defer l.Close()

	searchReq := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(Filter, user),
		[]string{},
		nil,
	)

	result, err := l.Search(searchReq)
	if err != nil {
		return fmt.Errorf("Search Error: %s", err)
	}

	if len(result.Entries) == 0 {
		return errors.New("User doesn't exist")
	}

	userdn := result.Entries[0].DN

	// Bind as the user to verify their password
	err = l.Bind(userdn, password)
	if err != nil {
		return err
	}

	return nil
}

// Query user's groups
func GetADGroups(config config.LDAPConfig, user string) ([]string, error) {
	var groups []string

	l, err := ConnectLDAP(config)

	if err != nil {
		log.Println(err)
		return nil, err
	}
	defer l.Close()

	searchReq := ldap.NewSearchRequest(
		config.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(Filter, user),
		[]string{},
		nil,
	)

	result, err := l.Search(searchReq)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	if len(result.Entries) == 0 {
		return nil, errors.New("User " + user + " doesn't exist")
	}

	entries := result.Entries[0].GetAttributeValues("memberOf")

	for _, entry := range entries {
		s := strings.Split(entry, "CN=")[1]
		s = strings.Split(strings.ToLower(s), ",")[0]
		groups = append(groups, s)
	}

	log.Println("user is member of: ", groups)

	return groups, nil
}
