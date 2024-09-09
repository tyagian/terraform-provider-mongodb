package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/megum1n/terraform-provider-mongodb/internal/mongodb"
)

var _ resource.Resource = &UserResource{}
var _ resource.ResourceWithImportState = &UserResource{}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

type UserResource struct {
	client *mongodb.Client
}

type UserResourceModel struct {
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
	Database types.String `tfsdk:"database"`
	Roles    types.Set    `tfsdk:"roles"`
}

func (r *UserResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *UserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "MongoDB User resource",

		Attributes: map[string]schema.Attribute{
			"username": schema.StringAttribute{
				MarkdownDescription: "Username",
				Required:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password",
				Required:            true,
				Sensitive:           true,
			},
			"database": schema.StringAttribute{
				MarkdownDescription: "Auth database name",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(defaultDatabase),
			},
			"roles": schema.SetNestedAttribute{
				MarkdownDescription: "Set of MongoDB roles",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"role": schema.StringAttribute{
							MarkdownDescription: "Role name",
							Required:            true,
						},
						"db": schema.StringAttribute{
							MarkdownDescription: "Target database name",
							Optional:            true,
							Computed:            true,
							Default:             stringdefault.StaticString(defaultDatabase),
						},
					},
				},
			},
		},
	}
}

func (r *UserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	var plan UserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var roles []mongodb.ShortRole
	resp.Diagnostics.Append(plan.Roles.ElementsAs(ctx, &roles, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.UpsertUser(ctx, &mongodb.User{
		Username: plan.Username.ValueString(),
		Password: plan.Password.ValueString(),
		Database: plan.Database.ValueString(),
		Roles:    roles,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"failed to upsert user",
			err.Error(),
		)

		return
	}

	tflog.Trace(ctx, "user created")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	var plan UserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.GetUser(ctx, &mongodb.GetUserOptions{
		Username: plan.Username.ValueString(),
		Database: plan.Database.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"failed to get user",
			err.Error(),
		)

		return
	}

	plan.Username = types.StringValue(user.Username)
	plan.Database = types.StringValue(user.Database)

	// Parse roles
	roles, d := user.Roles.ToTerraformSet(ctx)

	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.Roles = *roles

	// Append state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	var plan *UserResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var roles []mongodb.ShortRole
	resp.Diagnostics.Append(plan.Roles.ElementsAs(ctx, &roles, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.UpsertUser(ctx, &mongodb.User{
		Username: plan.Username.ValueString(),
		Password: plan.Password.ValueString(),
		Database: plan.Database.ValueString(),
		Roles:    roles,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"failed to upsert user",
			err.Error(),
		)

		return
	}

	tflog.Trace(ctx, "user updated")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	var plan UserResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteUser(ctx, &mongodb.DeleteUserOptions{
		Username: plan.Username.ValueString(),
		Database: plan.Database.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"failed to delete user",
			err.Error(),
		)
	}

	tflog.Trace(ctx, "user deleted")
	resp.State.RemoveResource(ctx)
}

func (r *UserResource) ImportState(
	ctx context.Context,
	req resource.ImportStateRequest,
	resp *resource.ImportStateResponse,
) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	idParts := strings.Split(req.ID, ".")

	var username, database string

	switch {
	case len(idParts) == 2:
		database = idParts[0]
		username = idParts[1]
	case len(idParts) == 1:
		username = idParts[0]
		database = defaultDatabase
	default:
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: '[<db>.]<username>'. Got: %q", req.ID),
		)

		return
	}

	plan := &UserResourceModel{}

	user, err := r.client.GetUser(ctx, &mongodb.GetUserOptions{
		Username: username,
		Database: database,
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to get user",
			err.Error(),
		)

		return
	}

	tflog.Debug(ctx, "TEST", map[string]interface{}{
		"user": user,
	})

	plan.Username = types.StringValue(user.Username)
	plan.Database = types.StringValue(user.Database)

	// Parse roles
	roles, d := user.Roles.ToTerraformSet(ctx)

	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.Roles = *roles

	// Append state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) checkClient(diag diag.Diagnostics) bool {
	if r.client == nil {
		diag.AddError(
			"MongoDB client is not configured",
			"Expected configured MongoDB client. Please report this issue to the provider developers.",
		)

		return false
	}

	return true
}
