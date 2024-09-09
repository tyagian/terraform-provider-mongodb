package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/megum1n/terraform-provider-mongodb/internal/mongodb"
)

var _ resource.Resource = &RoleResource{}
var _ resource.ResourceWithImportState = &RoleResource{}

func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

type RoleResource struct {
	client *mongodb.Client
}

type RoleResourceModel struct {
	ID          types.String `tfsdk:"id"`
	LastUpdated types.String `tfsdk:"last_updated"`

	Name           types.String `tfsdk:"name"`
	Database       types.String `tfsdk:"database"`
	InheritedRoles types.Set    `tfsdk:"inherited_role"`
	Privileges     types.Set    `tfsdk:"privileges"`
}

func (r *RoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Role resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"last_updated": schema.StringAttribute{
				Computed: true,
			},

			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the role",
				Required:            true,
			},
			"database": schema.StringAttribute{
				MarkdownDescription: "Role database name",
				Optional:            true,
				Default:             stringdefault.StaticString("admin"),
			},
			"inherited_role": schema.SetNestedAttribute{
				MarkdownDescription: "MongoDB inherited roles",
				Required:            false,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"role": schema.StringAttribute{
							MarkdownDescription: "Role name",
							Required:            true,
						},
						"db": schema.StringAttribute{
							MarkdownDescription: "Target database name",
							Required:            true,
						},
					},
				},
			},
			"privilege": schema.SetNestedAttribute{
				MarkdownDescription: "MongoDB role privileges",
				Required:            false,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"resource": schema.ObjectAttribute{
							AttributeTypes: map[string]attr.Type{
								"db":         types.StringType,
								"collection": types.StringType,
							},
							Required: true,
						},
						"actions": schema.SetAttribute{
							MarkdownDescription: "List of actions",
							ElementType:         types.StringType,
							Required:            true,
						},
					},
				},
			},
		},
	}
}

func (r *RoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*mongodb.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *mongodb.client, got: %T. "+
				"Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	var plan *RoleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var inheritedRoles []mongodb.ShortRole
	resp.Diagnostics.Append(plan.InheritedRoles.ElementsAs(ctx, &inheritedRoles, false)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var privileges []mongodb.Privilege
	resp.Diagnostics.Append(plan.Privileges.ElementsAs(ctx, &privileges, false)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.UpsertRole(ctx, &mongodb.Role{
		Name:       plan.Name.ValueString(),
		Database:   plan.Database.ValueString(),
		Privileges: privileges,
		Roles:      inheritedRoles,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"failed to upsert role",
			err.Error(),
		)

		return
	}

	tflog.Trace(ctx, "role created")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	var plan *RoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.client.GetRole(ctx, &mongodb.GetRoleOptions{
		Name:     plan.Name.ValueString(),
		Database: plan.Database.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"failed to get role",
			err.Error(),
		)

		return
	}

	plan.Name = types.StringValue(role.Name)
	plan.Database = types.StringValue(role.Database)

	// Parse inherited roles
	inheritedRoles, d := role.Roles.ToTerraformSet(ctx)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.InheritedRoles = *inheritedRoles

	// Parse privileges
	privileges, d := role.Privileges.ToTerraformSet(ctx)
	resp.Diagnostics.Append(d...)

	if resp.Diagnostics.HasError() {
		return
	}

	plan.Privileges = *privileges

	// Update state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	var plan *RoleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var inheritedRoles []mongodb.ShortRole
	resp.Diagnostics.Append(plan.InheritedRoles.ElementsAs(ctx, &inheritedRoles, false)...)

	if resp.Diagnostics.HasError() {
		return
	}

	var privileges []mongodb.Privilege
	resp.Diagnostics.Append(plan.Privileges.ElementsAs(ctx, &privileges, false)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.UpsertRole(ctx, &mongodb.Role{
		Name:       plan.Name.ValueString(),
		Database:   plan.Database.ValueString(),
		Privileges: privileges,
		Roles:      inheritedRoles,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"failed to upsert role",
			err.Error(),
		)

		return
	}

	tflog.Trace(ctx, "role updated")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	var data *RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteRole(ctx, &mongodb.DeleteRoleOptions{
		Name:     data.Name.ValueString(),
		Database: data.Database.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"failed to delete role",
			err.Error(),
		)
	}

	tflog.Trace(ctx, "role deleted")
	resp.State.RemoveResource(ctx)
}

func (r *RoleResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	idParts := strings.Split(req.ID, ",")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: name,db. Got: %q", req.ID),
		)

		return
	}

	var plan *RoleResourceModel

	role, err := r.client.GetRole(ctx, &mongodb.GetRoleOptions{
		Name:     idParts[0],
		Database: idParts[1],
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to get role",
			err.Error(),
		)

		return
	}

	plan.Name = types.StringValue(role.Name)
	plan.Database = types.StringValue(role.Database)

	// Parse inherited roles
	inheritedRoles, d := role.Roles.ToTerraformSet(ctx)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.InheritedRoles = *inheritedRoles

	// Parse privileges
	privileges, d := role.Privileges.ToTerraformSet(ctx)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.Privileges = *privileges

	// Append state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleResource) checkClient(diag diag.Diagnostics) bool {
	if r.client == nil {
		diag.AddError(
			"MongoDB client is not configured",
			"Expected configured MongoDB client. Please report this issue to the provider developers.",
		)

		return false
	}

	return true
}
