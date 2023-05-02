package infraflow

const (
	// MarkerMigratedFromTerraform is a marker to describe if the infrastructure has already migrated from Terraform.
	MarkerMigratedFromTerraform = "MigratedFromTerraform"
	// MarkerTerraformCleanedUp is a marker to describe if the Terraform resources are already cleaned up.
	MarkerTerraformCleanedUp = "TerraformCleanedUp"

	// ObjectKeyServiceAccount is the key to store the service account object.
	ObjectKeyServiceAccount = "service-account"
	// ObjectKeyVPC is the key to store the VPC object.
	ObjectKeyVPC = "vpc"
	// ObjectKeyNodeSubnet is the key to store the nodes subnet object.
	ObjectKeyNodeSubnet = "subnet-nodes"
	// ObjectKeyInternalSubnet is the key to store the internal subnet object.
	ObjectKeyInternalSubnet = "subnet-internal"
	// ObjectKeyRouter router is the key for the CloudRouter.
	ObjectKeyRouter = "router"
	// ObjectKeyNAT is the key for the .CloudNAT object.
	ObjectKeyNAT = "nat"
	// ObjectKeyIPAddress is the key for the IP Address slice.
	ObjectKeyIPAddress = "addresses/ip"
)
