package cloudcredentials

import (
	"github.com/rancher/norman/types"
)

type CloudCredential struct {
	types.Resource
	Annotations                  map[string]string             `json:"annotations,omitempty"`
	Created                      string                        `json:"created,omitempty"`
	CreatorID                    string                        `json:"creatorId,omitempty"`
	Description                  string                        `json:"description,omitempty"`
	Labels                       map[string]string             `json:"labels,omitempty"`
	Name                         string                        `json:"name,omitempty"`
	Removed                      string                        `json:"removed,omitempty"`
	AmazonEC2CredentialConfig    *AmazonEC2CredentialConfig    `json:"amazonec2credentialConfig,omitempty"`
	AzureCredentialConfig        *AzureCredentialConfig        `json:"azurecredentialConfig,omitempty"`
	DigitalOceanCredentialConfig *DigitalOceanCredentialConfig `json:"digitaloceancredentialConfig,omitempty"`
	LinodeCredentialConfig       *LinodeCredentialConfig       `json:"linodecredentialConfig,omitempty"`
	HarvesterCredentialConfig    *HarvesterCredentialConfig    `json:"harvestercredentialConfig,omitempty"`
	GoogleCredentialConfig       *GoogleCredentialConfig       `json:"googlecredentialConfig,omitempty"`
	UUID                         string                        `json:"uuid,omitempty"`
}
