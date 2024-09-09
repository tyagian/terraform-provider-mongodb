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
	ID          types.String `tfsdk:"id"`
	LastUpdated types.String `tfsdk:"last_updated"`

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
		MarkdownDescription: "User resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"last_updated": schema.StringAttribute{
				Computed: true,
			},

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
				Default:             stringdefault.StaticString("admin"),
			},
			"role": schema.SetNestedAttribute{
				MarkdownDescription: "MongoDB role",
				Required:            true,
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
		},
	}
}

func (r *UserResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
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

	tflog.Trace(ctx, "user created")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.checkClient(resp.Diagnostics) {
		return
	}

	var plan *UserResourceModel

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

	var plan *UserResourceModel

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

	idParts := strings.Split(req.ID, ",")

	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: username,db. Got: %q", req.ID),
		)

		return
	}

	var plan *UserResourceModel

	user, err := r.client.GetUser(ctx, &mongodb.GetUserOptions{
		Username: idParts[0],
		Database: idParts[1],
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to get user",
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
