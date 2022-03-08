package ec2

const ConfigurationFileKey = "awsEC2Config"

type AWSEC2Config struct {
	Region              string   `json:"region" yaml:"region"`
	InstanceTypeLinux   string   `json:"instanceTypeLinux" yaml:"instanceTypeLinux"`
	InstanceTypeWindows string   `json:"instanceTypeWindows" yaml:"instanceTypeWindows"`
	AWSRegionAZ         string   `json:"awsRegionAZ" yaml:"awsRegionAZ"`
	AWSLinuxAMI         string   `json:"awsLinuxAMI" yaml:"awsLinuxAMI"`
	AWSWindowsAMI       string   `json:"awsWindowsAMI" yaml:"awsWindowsAMI"`
	AWSSecurityGroups   []string `json:"awsSecurityGroups" yaml:"awsSecurityGroups"`
	AWSAccessKeyID      string   `json:"awsAccessKeyID" yaml:"awsAccessKeyID"`
	AWSSecretAccessKey  string   `json:"awsSecretAccessKey" yaml:"awsSecretAccessKey"`
	AWSSSHKeyName       string   `json:"awsSSHKeyName" yaml:"awsSSHKeyName"`
	AWSCICDInstanceTag  string   `json:"awsCICDInstanceTag" yaml:"awsCICDInstanceTag"`
	AWSIAMProfile       string   `json:"awsIAMProfile" yaml:"awsIAMProfile"`
	AWSUser             string   `json:"awsUser" yaml:"awsUser"`
	VolumeSizeLinux     int      `json:"volumeSizeLinux" yaml:"volumeSizeLinux"`
	VolumeSizeWindows   int      `json:"volumeSizeWindows" yaml:"volumeSizeWindows"`
}
