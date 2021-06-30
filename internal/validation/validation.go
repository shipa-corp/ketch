// Package validation provide utilities functions for data validation.
package validation

import (
	"net"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
)

var (
	nameRegexp         = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)
	yamlFilenameRegexp = regexp.MustCompile(`^[A-Za-z0-9-_\/\.]{1,255}\.(yaml|yml)$`)
)

// Error represents the package's Error type that is returned by Validate* functions.
type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrInvalidCname        Error = "invalid cname"
	ErrIPAddress           Error = "invalid cname: cname must be a DNS name, not an IP address"
	ErrInvalidWildcard     Error = "invalid cname: a wildcard cname must start with '*.', followed by a valid DNS subdomain, which must consist of lower case alphanumeric characters, '-' or '.' and end with an alphanumeric character"
	ErrInvalidDnsSubdomain Error = "invalid cname: cname must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character"
)

// ValidateName checks whether the given name is valid.
func ValidateName(name string) bool {
	return nameRegexp.MatchString(name)
}

// ValidateCname checks whether the given CNAME is valid.
func ValidateCname(cname string) error {
	cnameRegexp := regexp.MustCompile(`^(\*\.)?[a-zA-Z0-9][\w-.]+$`)
	if !cnameRegexp.MatchString(cname) {
		return ErrInvalidCname
	}
	// we create an ingress object for an application and assign a cname to Ingress.Rules.Host.
	// the following validation is taken from
	// https://github.com/kubernetes/kubernetes/blob/release-1.19/pkg/apis/networking/validation/validation.go#L323
	if isIP := (net.ParseIP(cname) != nil); isIP {
		return ErrIPAddress
	}
	if strings.Contains(cname, "*") {
		msgs := validation.IsWildcardDNS1123Subdomain(cname)
		if len(msgs) > 0 {
			return ErrInvalidWildcard
		}
	} else {
		msgs := validation.IsDNS1123Subdomain(cname)
		if len(msgs) > 0 {
			return ErrInvalidDnsSubdomain
		}
	}
	return nil
}

func ValidateYamlFilename(name string) bool {
	return yamlFilenameRegexp.MatchString(name)
}
