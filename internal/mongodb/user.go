package mongodb

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
)

const (
	createUserCmd = "createUser"
	getUserCmd    = "usersInfo"
	updateUserCmr = "updateUser"
	deleteUserCmd = "dropUser"
)

func (c *Client) UpsertUser(ctx context.Context, user *User) (*User, error) {
	var cmd string

	getUserOptions := &GetUserOptions{
		Username: user.Username,
		Database: user.Database,
	}
	_, err := c.GetUser(ctx, getUserOptions)

	switch {
	case errors.As(err, &NotFoundError{}):
		cmd = createUserCmd
	case err == nil:
		cmd = updateUserCmr
	default:
		return nil, err
	}

	command := bson.M{
		cmd:     user.Username,
		"pwd":   user.Password,
		"roles": user.Roles,
	}

	response := c.mongo.Database(user.Database).RunCommand(ctx, command)
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

	user, err = c.GetUser(ctx, getUserOptions)
	if err != nil {
		return nil, err
	}

	return user, nil
}

type GetUserOptions struct {
	Username string
	Database string
}

func (c *Client) GetUser(ctx context.Context, options *GetUserOptions) (*User, error) {
	command := bson.M{getUserCmd: options.Username}

	response := c.mongo.Database(options.Database).RunCommand(ctx, command)
	if err := response.Err(); err != nil {
		return nil, err
	}

	var result = struct {
		Result
		Users []User `bson:"users"`
	}{}

	err := response.Decode(&result)
	if err != nil {
		return nil, err
	}

	if result.Ok != 1 {
		return nil, FailedCommandError{getUserCmd}
	}

	userCount := len(result.Users)

	switch {
	case userCount == 0:
		return nil, NotFoundError{}
	case userCount > 1:
		return nil, TooManyError{t: "user"}
	}

	return &result.Users[0], nil
}

type DeleteUserOptions struct {
	Username string
	Database string
}

func (c *Client) DeleteUser(ctx context.Context, options *DeleteUserOptions) error {
	command := bson.M{deleteUserCmd: options.Username}

	response := c.mongo.Database(options.Database).RunCommand(ctx, command)
	if err := response.Err(); err != nil {
		return err
	}

	result := Result{}

	err := response.Decode(&result)
	if err != nil {
		return err
	}

	if result.Ok != 1 {
		return FailedCommandError{deleteUserCmd}
	}

	return nil
}
