package mongodb

import (
	"fmt"
)

type NotFoundError struct {
	name string
	t    string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("%s %s not found", e.name, e.t)
}

type TooManyError struct {
	t string
}

func (e TooManyError) Error() string {
	return fmt.Sprintf("found too many %ss", e.t)
}

type FailedCommandError struct {
	Cmd string
}

func (e FailedCommandError) Error() string {
	return e.Cmd + " command failed"
}
