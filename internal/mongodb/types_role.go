package mongodb

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type Resource struct {
	DB         string `bson:"db"         tfsdk:"db"`
	Collection string `bson:"collection" tfsdk:"collection"`
}

type Privilege struct {
	Resource Resource `bson:"resource" tfsdk:"resource"`
	Actions  []string `bson:"actions"  tfsdk:"actions"`
}

type Privileges []Privilege

func (p *Privileges) ToTerraformSet(ctx context.Context) (*types.Set, diag.Diagnostics) {
	privileges := make([]basetypes.ObjectValue, 0, len(*p))

	privilegeType := types.ObjectType{
		AttrTypes: PrivilegeAttributeTypes,
	}

	for _, privilege := range *p {
		privilegeObject, d := types.ObjectValueFrom(ctx, PrivilegeAttributeTypes, privilege)

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
	DB   string `bson:"db"   tfsdk:"db"`
}

type ShortRoles []ShortRole

func (r *ShortRoles) ToTerraformSet(ctx context.Context) (*types.Set, diag.Diagnostics) {
	roles := make([]basetypes.ObjectValue, 0, len(*r))

	roleType := types.ObjectType{
		AttrTypes: ShortRoleAttributeTypes,
	}

	for _, role := range *r {
		roleObject, d := types.ObjectValueFrom(ctx, ShortRoleAttributeTypes, role)

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

var ShortRoleAttributeTypes = map[string]attr.Type{
	"role": types.StringType,
	"db":   types.StringType,
}

var PrivilegeAttributeTypes = map[string]attr.Type{
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
