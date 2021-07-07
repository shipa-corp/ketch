package main

import (
	"errors"
)

type cliError string

func (e cliError) Error() string { return string(e) }

const (
	ErrInvalidFrameworkName cliError = "invalid framework, arg should be a <framework>.yaml file or specify a framework name that has" +
		"at most 40 characters, containing only lower case letters, numbers or dashes, starting with a letter"

	ErrInvalidAppName cliError = "invalid app name, app name should have at most 40 " +
		"characters, containing only lower case letters, numbers or dashes, starting with a letter"

	ErrNoEntrypointAndCmd   cliError = "image doesn't have entrypoint and cmd set"
	ErrLogUnknownTimeFormat cliError = "unknown time format"

	ErrClusterIssuerNotFound cliError = "cluster issuer not found"
)

func unwrappedError(err error) error {
	for {
		if errors.Unwrap(err) == nil {
			return err
		}
		err = errors.Unwrap(err)
	}
}
