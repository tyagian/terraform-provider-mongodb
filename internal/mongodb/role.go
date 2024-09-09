package mongodb

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
)

const (
	createRoleCmd = "createRole"
	getRoleCmd    = "rolesInfo"
	updateRoleCmd = "updateRole"
	deleteRoleCmd = "dropRole"
)

func (c *Client) UpsertRole(ctx context.Context, role *Role) (*Role, error) {
	var cmd string

	_, err := c.GetRole(ctx, &GetRoleOptions{
		Name:     role.Name,
		Database: role.Database,
	})

	switch {
	case errors.As(err, &NotFoundError{}):
		cmd = createRoleCmd
	case err == nil:
		cmd = updateRoleCmd
	default:
		return nil, err
	}

	command := bson.M{
		cmd:          role.Name,
		"privileges": role.Privileges,
		"roles":      role.Roles,
	}

	response := c.mongo.Database(role.Database).RunCommand(ctx, command)
	if err = response.Err(); err != nil {
		return nil, err
	}

	result := &Result{}

	err = response.Decode(result)
	if err != nil {
		return nil, err
	}

	if result.Ok != 1 {
		return nil, FailedCommandError{cmd}
	}

	role, err = c.GetRole(ctx, &GetRoleOptions{
		Name:     role.Name,
		Database: role.Database,
	})
	if err != nil {
		return nil, err
	}

	return role, nil
}

type GetRoleOptions struct {
	Name     string
	Database string
}

func (c *Client) GetRole(ctx context.Context, options *GetRoleOptions) (*Role, error) {
	command := bson.M{
		getRoleCmd:       options.Name,
		"showPrivileges": true,
	}

	response := c.mongo.Database(options.Database).RunCommand(ctx, command)
	if err := response.Err(); err != nil {
		return nil, err
	}

	result := struct {
		Result
		Roles []Role `bson:"roles"`
	}{}

	err := response.Decode(&result)
	if err != nil {
		return nil, err
	}

	if result.Ok != 1 {
		return nil, FailedCommandError{getRoleCmd}
	}

	roleCount := len(result.Roles)

	switch {
	case roleCount == 0:
		return nil, NotFoundError{options.Name, "role"}
	case roleCount > 1:
		return nil, TooManyError{"role"}
	}

	return &result.Roles[0], nil
}

type DeleteRoleOptions struct {
	Name     string
	Database string
}

func (c *Client) DeleteRole(ctx context.Context, options *DeleteRoleOptions) error {
	command := bson.M{deleteRoleCmd: options.Name}

	response := c.mongo.Database(options.Database).RunCommand(ctx, command)
	if err := response.Err(); err != nil {
		return err
	}

	var result Result

	err := response.Decode(&result)
	if err != nil {
		return err
	}

	if result.Ok != 1 {
		return FailedCommandError{deleteRoleCmd}
	}

	return nil
}
