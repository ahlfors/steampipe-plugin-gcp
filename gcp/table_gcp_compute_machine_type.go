package gcp

import (
	"context"
	"strings"

	"github.com/turbot/go-kit/types"
	"github.com/turbot/steampipe-plugin-sdk/v5/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin/transform"
	"google.golang.org/api/compute/v1"
)

//// TABLE DEFINITION

func tableGcpComputeMachineType(ctx context.Context) *plugin.Table {
	return &plugin.Table{
		Name:        "gcp_compute_machine_type",
		Description: "GCP Compute Machine Type",
		Get: &plugin.GetConfig{
			KeyColumns: plugin.SingleColumn("name"),
			Hydrate:    getComputeMachineType,
		},
		List: &plugin.ListConfig{
			Hydrate: listComputeMachineTypes,
		},
		Columns: []*plugin.Column{
			{
				Name:        "name",
				Description: "Name of the resource.",
				Type:        proto.ColumnType_STRING,
			},
			{
				Name:        "id",
				Description: "An unique identifier for the resource. This identifier is defined by the server.",
				Type:        proto.ColumnType_INT,
			},
			{
				Name:        "self_link",
				Description: "Server-defined URL for the resource.",
				Type:        proto.ColumnType_STRING,
			},
			{
				Name:        "creation_timestamp",
				Description: "Creation timestamp in RFC3339 text format.",
				Type:        proto.ColumnType_TIMESTAMP,
				Transform:   transform.FromGo().NullIfZero(),
			},
			{
				Name:        "description",
				Description: "An optional textual description of the resource.",
				Type:        proto.ColumnType_STRING,
			},
			{
				Name:        "guest_cpus",
				Description: "The number of virtual CPUs that are available to the instance.",
				Type:        proto.ColumnType_INT,
			},
			{
				Name:        "memory_mb",
				Description: "The amount of physical memory available to the instance, defined in MB.",
				Type:        proto.ColumnType_INT,
			},
			{
				Name:        "image_space_gb",
				Description: "The amount of memory available for image ig GB.",
				Type:        proto.ColumnType_INT,
			},
			{
				Name:        "maximum_persistent_disks",
				Description: "Maximum persistent disks allowed.",
				Type:        proto.ColumnType_INT,
			},
			{
				Name:        "maximum_persistent_disks_size_gb",
				Description: "Maximum total persistent disks size (GB) allowed.",
				Type:        proto.ColumnType_INT,
			},
			{
				Name:        "is_shared_cpu",
				Description: "Whether this machine type has a shared CPU.",
				Type:        proto.ColumnType_BOOL,
			},
			{
				Name:        "kind",
				Description: "The type of the resource. Always compute#machineType for machine types.",
				Type:        proto.ColumnType_STRING,
			},
			{
				Name:        "accelerators",
				Description: "A list of accelerator configurations assigned to this machine type.",
				Type:        proto.ColumnType_JSON,
			},
			{
				Name:        "scratch_disks",
				Description: "A list of extended scratch disks assigned to the instance.",
				Type:        proto.ColumnType_JSON,
			},

			// Steampipe standard columns
			{
				Name:        "title",
				Description: ColumnDescriptionTitle,
				Type:        proto.ColumnType_STRING,
				Transform:   transform.FromField("Name"),
			},
			{
				Name:        "akas",
				Description: ColumnDescriptionAkas,
				Type:        proto.ColumnType_JSON,
				Transform:   transform.FromP(machineTypeTurbotData, "Akas"),
			},

			// GCP standard columns
			{
				Name:        "project",
				Description: ColumnDescriptionProject,
				Type:        proto.ColumnType_STRING,
				Transform:   transform.FromP(machineTypeTurbotData, "Project"),
			},
		},
	}
}

//// LIST FUNCTION

func listComputeMachineTypes(ctx context.Context, d *plugin.QueryData, h *plugin.HydrateData) (interface{}, error) {
	plugin.Logger(ctx).Trace("listComputeMachineTypes")

	// Create Service Connection
	service, err := ComputeService(ctx, d)
	if err != nil {
		return nil, err
	}

	// Max limit is set as per documentation
	// https://pkg.go.dev/google.golang.org/api@v0.48.0/compute/v1?utm_source=gopls#MachineTypesListCall.MaxResults
	pageSize := types.Int64(500)
	limit := d.QueryContext.Limit
	if d.QueryContext.Limit != nil {
		if *limit < *pageSize {
			pageSize = limit
		}
	}

	// Get project details
	getProjectCached := plugin.HydrateFunc(getProject).WithCache()
	projectId, err := getProjectCached(ctx, d, h)
	if err != nil {
		return nil, err
	}
	project := projectId.(string)
	zone := "us-central1-a"

	resp := service.MachineTypes.List(project, zone).MaxResults(*pageSize)
	if err := resp.Pages(ctx, func(page *compute.MachineTypeList) error {
		for _, machineType := range page.Items {
			d.StreamListItem(ctx, machineType)

			// Check if context has been cancelled or if the limit has been hit (if specified)
			// if there is a limit, it will return the number of rows required to reach this limit
			if d.RowsRemaining(ctx) == 0 {
				page.NextPageToken = ""
				return nil
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return nil, nil
}

//// HYDRATE FUNCTIONS

func getComputeMachineType(ctx context.Context, d *plugin.QueryData, h *plugin.HydrateData) (interface{}, error) {
	plugin.Logger(ctx).Trace("getComputeMachineType")
	// Create Service Connection
	service, err := ComputeService(ctx, d)
	if err != nil {
		return nil, err
	}

	// Get project details
	getProjectCached := plugin.HydrateFunc(getProject).WithCache()
	projectId, err := getProjectCached(ctx, d, h)
	if err != nil {
		return nil, err
	}
	project := projectId.(string)
	zone := "us-central1-a"
	machineTypeName := d.EqualsQuals["name"].GetStringValue()

	// Return nil, if no input provided
	if machineTypeName == "" {
		return nil, nil
	}

	resp, err := service.MachineTypes.Get(project, zone, machineTypeName).Do()
	if err != nil {
		return nil, err
	}
	return resp, nil
}

//// TRANSFORM FUNCTIONS

func machineTypeTurbotData(_ context.Context, d *transform.TransformData) (interface{}, error) {
	data := d.HydrateItem.(*compute.MachineType)
	param := d.Param.(string)

	project := strings.Split(data.SelfLink, "/")[6]

	turbotData := map[string]interface{}{
		"Project": project,
		"Akas":    []string{"gcp://compute.googleapis.com/projects/" + project + "/machineTypes/" + data.Name},
	}

	return turbotData[param], nil
}
