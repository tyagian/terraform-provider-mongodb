package mongodb

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"go.mongodb.org/mongo-driver/bson"
)

type Resource struct {
	DB         string `bson:"db"`
	Collection string `bson:"collection"`
}

type Privilege struct {
	Resource Resource `bson:"resource"`
	Actions  []string `bson:"actions"`
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

func (p *Privileges) toBson() bson.A {
	out := bson.A{}

	for _, privilege := range *p {
		out = append(out, bson.M{
			"resource": bson.M{
				"db":         privilege.Resource.DB,
				"collection": privilege.Resource.Collection,
			},
			"actions": privilege.Actions,
		})
	}

	return out
}

type ShortRole struct {
	Role string `bson:"role" tfsdk:"role"`
	DB   string `bson:"db" tfsdk:"db"`
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

func (r *ShortRoles) toBson() bson.A {
	out := bson.A{}

	for _, role := range *r {
		out = append(out, bson.M{"role": role.Role, "db": role.DB})
	}

	return out
}

type Role struct {
	Name       string     `bson:"role"`
	Database   string     `bson:"db"`
	Privileges Privileges `bson:"privileges"`
	Roles      ShortRoles `bson:"roles"`
}

var shortRoleAttributeTypes = map[string]attr.Type{
	"role": types.StringType,
	"db":   types.StringType,
}

var privilegeAttributeTypes = map[string]attr.Type{
	"resource": types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"db":         types.StringType,
			"collection": types.StringType,
		},
	},
	"actions": types.SetType{
		ElemType: types.StringType,
	},
}
