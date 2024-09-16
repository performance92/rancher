# Backup Restore Operator (BRO)
Backup Restore Operator (BRO for short) is a disaster recovery chart that can be installed on the local cluster only, and is used to create backups of Rancher resources contained in the `rancher-resource-set`. The resource set becomes available after the chart is installed and can be edited as a user needs. Once a backup is created and stored in either a local volume or in an S3 storage location (such as AWS S3 or Minio S3), a restore operation can be created to restore Rancher back to the backed up version of the Rancher resources.

## Pre-requisites
- Downstream RKE1 and RKE2 clusters are provisioned during the test to ensure the clusters come back into the active status after a restore.
- Provisioning config is required because of this, example provisioningInput included below.
- All tests require configs pulled in from the backupRestoreInput parameter.

## Test Setup
In your config file, set the following:
```
rancher: 
  host: ""
  adminToken: ""
  insecure: true/false
  cleanup: true/false
  clusterName: ""
backupRestoreInput:
  backupName: ""
  s3BucketName: ""
  s3FolderName: ""
  s3Region: ""
  s3Endpoint: ""
  volumeName: "" # Optional
  credentialSecretNamespace: ""
  prune: true/false
  resourceSetName: ""
  accessKey: ""
  secretKey: ""
provisioningInput:
  rke1KubernetesVersion:
    - ""
  rke2KubernetesVersion:
    - ""
  k3sKubernetesVersion:
    - ""
  nodeProviders:
    - ""

awsEC2Configs:
  region: ""
  awsSecretAccessKey: ""
  awsAccessKeyID: ""
  awsEC2Config:
    - instanceType: ""
      awsRegionAZ: ""
      awsAMI: ""
      awsSecurityGroups: [""]
      awsSSHKeyName: ""
      awsCICDInstanceTag: ""
      awsIAMProfile: ""
      awsUser: ""
      volumeSize: # int
      roles: ["", "", ""] # etcd, controlplane, and worker are the options
      isWindows: true/false

sshPath:
  sshPath: ""
```