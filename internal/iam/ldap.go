package iam

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// LDAPConfig holds LDAP authentication configuration.
type LDAPConfig struct {
	ServerURL      string            `json:"server_url" yaml:"server_url"`
	BindDN         string            `json:"bind_dn" yaml:"bind_dn"`
	BindPassword   string            `json:"bind_password" yaml:"bind_password"`
	BaseDN         string            `json:"base_dn" yaml:"base_dn"`
	UserFilter     string            `json:"user_filter" yaml:"user_filter"` // e.g. "(uid=%s)"
	GroupAttr       string            `json:"group_attr" yaml:"group_attr"` // e.g. "memberOf"
	GroupPolicyMap  map[string]string `json:"group_policy_map" yaml:"group_policy_map"` // group DN → policy name
	TLSSkipVerify  bool              `json:"tls_skip_verify" yaml:"tls_skip_verify"`
	StartTLS       bool              `json:"start_tls" yaml:"start_tls"`
}

// LDAPAuth provides LDAP-based authentication.
type LDAPAuth struct {
	cfg LDAPConfig
}

// NewLDAPAuth creates a new LDAP authenticator.
func NewLDAPAuth(cfg LDAPConfig) *LDAPAuth {
	return &LDAPAuth{cfg: cfg}
}

// Authenticate verifies username/password against LDAP and returns mapped policy names.
func (l *LDAPAuth) Authenticate(username, password string) ([]string, error) {
	conn, err := l.connect()
	if err != nil {
		return nil, fmt.Errorf("ldap connect: %w", err)
	}
	defer conn.Close()

	// Bind with service account to search
	if l.cfg.BindDN != "" {
		if err := conn.Bind(l.cfg.BindDN, l.cfg.BindPassword); err != nil {
			return nil, fmt.Errorf("ldap service bind: %w", err)
		}
	}

	// Search for user
	filter := strings.ReplaceAll(l.cfg.UserFilter, "%s", ldap.EscapeFilter(username))
	attrs := []string{"dn"}
	if l.cfg.GroupAttr != "" {
		attrs = append(attrs, l.cfg.GroupAttr)
	}

	sr, err := conn.Search(ldap.NewSearchRequest(
		l.cfg.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, 0, false,
		filter,
		attrs,
		nil,
	))
	if err != nil {
		return nil, fmt.Errorf("ldap search: %w", err)
	}
	if len(sr.Entries) == 0 {
		return nil, fmt.Errorf("ldap user not found")
	}

	userDN := sr.Entries[0].DN

	// Bind as user to verify password
	if err := conn.Bind(userDN, password); err != nil {
		return nil, fmt.Errorf("ldap user bind: %w", err)
	}

	// Extract groups and map to policies
	var policies []string
	if l.cfg.GroupAttr != "" && len(l.cfg.GroupPolicyMap) > 0 {
		groups := sr.Entries[0].GetAttributeValues(l.cfg.GroupAttr)
		for _, g := range groups {
			if policy, ok := l.cfg.GroupPolicyMap[g]; ok {
				policies = append(policies, policy)
			}
		}
	}

	return policies, nil
}

func (l *LDAPAuth) connect() (*ldap.Conn, error) {
	if strings.HasPrefix(l.cfg.ServerURL, "ldaps://") {
		tlsCfg := &tls.Config{InsecureSkipVerify: l.cfg.TLSSkipVerify}
		return ldap.DialURL(l.cfg.ServerURL, ldap.DialWithTLSConfig(tlsCfg))
	}

	conn, err := ldap.DialURL(l.cfg.ServerURL)
	if err != nil {
		return nil, err
	}

	if l.cfg.StartTLS {
		tlsCfg := &tls.Config{InsecureSkipVerify: l.cfg.TLSSkipVerify}
		if err := conn.StartTLS(tlsCfg); err != nil {
			conn.Close()
			return nil, err
		}
	}

	return conn, nil
}
