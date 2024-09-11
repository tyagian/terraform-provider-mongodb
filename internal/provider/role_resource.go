package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/megum1n/terraform-provider-mongodb/internal/mongodb"
)

var _ resource.Resource = &RoleResource{}
var _ resource.ResourceWithConfigure = &RoleResource{}
var _ resource.ResourceWithImportState = &RoleResource{}
var _ resource.ResourceWithConfigValidators = &RoleResource{}

func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

type RoleResource struct {
	client *mongodb.Client
}

type RoleResourceModel struct {
	Name       types.String `tfsdk:"name"`
	Database   types.String `tfsdk:"database"`
	Roles      types.Set    `tfsdk:"roles"`
	Privileges types.Set    `tfsdk:"privileges"`
}

func (r *RoleResourceModel) UpdateState(ctx context.Context, role *mongodb.Role) diag.Diagnostics {
	diags := diag.Diagnostics{}

	r.Name = types.StringValue(role.Name)
	r.Database = types.StringValue(role.Database)

	// Parse roles
	roles, d := role.Roles.ToTerraformSet(ctx)
	diags.Append(d...)
	r.Roles = *roles

	// Parse privileges
	privileges, d := role.Privileges.ToTerraformSet(ctx)
	diags.Append(d...)
	r.Privileges = *privileges

	return diags
}

func (r *RoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "MongoDB Role resource",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "The name of the new role",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"database": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("Target database name. "+
					"%q is used by default", defaultDatabase),
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(defaultDatabase),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"roles": schema.SetNestedAttribute{
				MarkdownDescription: "Set of roles from which this role inherits privileges",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"role": schema.StringAttribute{
							MarkdownDescription: "Role name",
							Required:            true,
						},
						"db": schema.StringAttribute{
							MarkdownDescription: fmt.Sprintf("Target database name. "+
								"%q is used by default", defaultDatabase),
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(defaultDatabase),
						},
					},
				},
			},
			"privileges": schema.SetNestedAttribute{
				MarkdownDescription: "Set of the privileges to grant the role",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"resource": schema.ObjectAttribute{
							MarkdownDescription: "A document that specifies the resources " +
								"upon which the privilege actions apply",
							AttributeTypes: map[string]attr.Type{
								"db":         types.StringType,
								"collection": types.StringType,
							},
							Required: true,
						},
						"actions": schema.SetAttribute{
							MarkdownDescription: "An array of actions permitted on the resource",
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

	p, ok := req.ProviderData.(*MongodbProvider)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *MongodbProvider, got: %T. "+
				"Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = p.client
}

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	var plan RoleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse roles
	var roles []mongodb.ShortRole

	resp.Diagnostics.Append(plan.Roles.ElementsAs(ctx, &roles, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse privileges
	var privileges []mongodb.Privilege

	resp.Diagnostics.Append(plan.Privileges.ElementsAs(ctx, &privileges, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.client.UpsertRole(ctx, &mongodb.Role{
		Name:       plan.Name.ValueString(),
		Database:   plan.Database.ValueString(),
		Privileges: privileges,
		Roles:      roles,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"failed to upsert role",
			err.Error(),
		)

		return
	}

	resp.Diagnostics.Append(plan.UpdateState(ctx, role)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "role created")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	var plan RoleResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.client.GetRole(ctx, &mongodb.GetRoleOptions{
		Name:     plan.Name.ValueString(),
		Database: plan.Database.ValueString(),
	})
	if err != nil {
		notFound := &mongodb.NotFoundError{}

		if !errors.As(err, &notFound) {
			resp.Diagnostics.AddError(
				"failed to get role",
				err.Error(),
			)

			return
		}

		tflog.Debug(ctx, "role not found, removing from state")
		resp.State.RemoveResource(ctx)

		return
	}

	resp.Diagnostics.Append(plan.UpdateState(ctx, role)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	var plan RoleResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse roles
	var roles []mongodb.ShortRole

	resp.Diagnostics.Append(plan.Roles.ElementsAs(ctx, &roles, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Parse privileges
	var privileges []mongodb.Privilege

	resp.Diagnostics.Append(plan.Privileges.ElementsAs(ctx, &privileges, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.client.UpsertRole(ctx, &mongodb.Role{
		Name:       plan.Name.ValueString(),
		Database:   plan.Database.ValueString(),
		Privileges: privileges,
		Roles:      roles,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"failed to upsert role",
			err.Error(),
		)

		return
	}

	resp.Diagnostics.Append(plan.UpdateState(ctx, role)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(ctx, "role updated")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	var plan RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteRole(ctx, &mongodb.DeleteRoleOptions{
		Name:     plan.Name.ValueString(),
		Database: plan.Database.ValueString(),
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

	idParts := strings.Split(req.ID, ".")

	var name, database string

	switch {
	case len(idParts) == 2:
		database = idParts[0]
		name = idParts[1]
	case len(idParts) == 1:
		name = idParts[0]
		database = defaultDatabase
	default:
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: '[<db>.]<role>'. Got: %q", req.ID),
		)

		return
	}

	plan := RoleResourceModel{}

	role, err := r.client.GetRole(ctx, &mongodb.GetRoleOptions{
		Name:     name,
		Database: database,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to get role",
			err.Error(),
		)

		return
	}

	resp.Diagnostics.Append(plan.UpdateState(ctx, role)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot("roles"),
			path.MatchRoot("privileges"),
		),
	}
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
