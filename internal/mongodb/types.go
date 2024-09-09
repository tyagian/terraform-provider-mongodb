package mongodb

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

type User struct {
	Username string `bson:"user"`
	Password string

	Database string     `bson:"db"`
	Roles    ShortRoles `bson:"roles"`
}

type RoleResource struct {
	DB         string `bson:"db"`
	Collection string `bson:"collection"`
}

type Privilege struct {
	Resource RoleResource `bson:"resource"`
	Actions  []string     `bson:"actions"`
}

var privilegeAttributeTypes = map[string]attr.Type{
	"resource": types.StringType,
	"actions":  types.SetType{},
}

type Privileges []Privilege

func (p *Privileges) ToTerraformSet(ctx context.Context) (*types.Set, diag.Diagnostics) {
	var privileges []basetypes.ObjectValue

	privilegeType := types.ObjectType{
		AttrTypes: privilegeAttributeTypes,
	}

	for _, privilege := range *p {
		privilegeObject, d := types.ObjectValueFrom(ctx, privilegeAttributeTypes, privilege)

		if d.HasError() {
			return nil, d
		}

		privileges = append(privileges, privilegeObject)
	}

	privilegesList, d := types.SetValueFrom(ctx, privilegeType, privileges)
	if d.HasError() {
		return nil, d
	}

	return &privilegesList, nil
}

type ShortRole struct {
	Role string `bson:"role"`
	DB   string `bson:"db"`
}

var shortRoleAttributeTypes = map[string]attr.Type{
	"role":     types.StringType,
	"database": types.StringType,
}

type ShortRoles []ShortRole

func (r *ShortRoles) ToTerraformSet(ctx context.Context) (*types.Set, diag.Diagnostics) {
	var roles []basetypes.ObjectValue

	roleType := types.ObjectType{
		AttrTypes: shortRoleAttributeTypes,
	}

	for _, role := range *r {
		roleObject, d := types.ObjectValueFrom(ctx, shortRoleAttributeTypes, role)

		if d.HasError() {
			return nil, d
		}

		roles = append(roles, roleObject)
	}

	rolesList, d := types.SetValueFrom(ctx, roleType, roles)
	if d.HasError() {
		return nil, d
	}

	return &rolesList, nil
}

type Role struct {
	Name       string     `bson:"role"`
	Database   string     `bson:"db"`
	Privileges Privileges `bson:"privileges"`
	Roles      ShortRoles `bson:"roles"`
}

type Result struct {
	Ok int `bson:"ok"`
}
