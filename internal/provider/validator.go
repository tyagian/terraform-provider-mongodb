package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const (
	externalDatabase = "$external"
)

var _ resource.ConfigValidator = userPasswordValidator{}

type userPasswordValidator struct{}

func (v userPasswordValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

func (v userPasswordValidator) MarkdownDescription(_ context.Context) string {
	return fmt.Sprintf("Password must be empty for %q database. "+
		"For other databases password must be set.", externalDatabase)
}

func (v userPasswordValidator) ValidateResource(
	ctx context.Context,
	req resource.ValidateConfigRequest,
	resp *resource.ValidateConfigResponse,
) {
	var user UserResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &user)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if user.Database.ValueString() != externalDatabase && user.Password.ValueString() == "" {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic(
			"Invalid user configuration",
			v.Description(ctx),
		))

		return
	}

	if user.Database.ValueString() == externalDatabase && user.Password.ValueString() != "" {
		resp.Diagnostics.Append(diag.NewErrorDiagnostic(
			"Invalid user configuration",
			v.Description(ctx),
		))

		return
	}
}
