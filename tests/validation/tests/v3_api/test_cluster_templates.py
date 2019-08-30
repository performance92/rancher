import os
import pytest
from .common import *  # NOQA
from .test_bkp_restore import wait_for_backup_to_active
from .test_rbac import create_user
from rancher import ApiError

DO_ACCESSKEY = os.environ.get('DO_ACCESSKEY', "None")
RANCHER_S3_BUCKETNAME = os.environ.get('RANCHER_S3_BUCKETNAME', "None")
RANCHER_S3_ENDPOINT = os.environ.get('RANCHER_S3_ENDPOINT', "None")
AWS_ACCESS_KEY_ID = os.environ.get('AWS_ACCESS_KEY_ID', "None")
AWS_SECRET_ACCESS_KEY = os.environ.get('AWS_SECRET_ACCESS_KEY', "None")

user_token = {"user_standard": {"user": None, "token": None},
              "newuser_standard": {"user": None, "token": None}}


@pytest.fixture(scope='module', autouse="True")
def setup(request):

    client = get_admin_client()

    # create users
    user_token["user_standard"]["user"], \
        user_token["user_standard"]["token"] = create_user(client)
    user_token["newuser_standard"]["user"], \
        user_token["newuser_standard"]["token"] = create_user(client)

    user_standard_id = user_token["user_standard"]["user"].id

    # Add clustertemplates-create global role binding to the standard user
    client.create_global_role_binding(globalRoleId="clustertemplates-create",
                                      subjectKind="User",
                                      userId=user_standard_id)


def get_k8s_versionlist():

    # Get the list of K8s version supported by the rancher server
    headers = {"Content-Type": "application/json",
               "Accept": "application/json",
               "Authorization": "Bearer " + ADMIN_TOKEN}
    json_data = {
        'responseType': 'json'
    }
    settings_url = CATTLE_TEST_URL + "/v3/settings/k8s-versions-current"
    response = requests.get(settings_url, json=json_data,
                            verify=False, headers=headers)
    json_response = (json.loads(response.content))
    k8sversionstring = json_response['value']
    k8sversionlist = k8sversionstring.split(",")
    assert len(k8sversionlist) > 1
    return k8sversionlist


def get_cluster_config(k8sversion):

    rke_config = getRKEConfig(k8sversion)
    cluster_config = {
        "dockerRootDir": "/var/lib/docker123",
        "enableClusterAlerting": "false",
        "enableClusterMonitoring": "false",
        "enableNetworkPolicy": "false",
        "type": "clusterSpecBase",
        "localClusterAuthEndpoint": {
            "enabled": "true",
            "type": "localClusterAuthEndpoint"
        },
        "rancherKubernetesEngineConfig": rke_config
    }
    return cluster_config


def test_cluster_template_create_with_questions():

    # Create a cluster template and revision with questions and create a
    # cluster with the revision
    k8sversionlist = get_k8s_versionlist()
    cluster_config = get_cluster_config(k8sversionlist[0])

    questions = [{
        "variable": "rancherKubernetesEngineConfig.kubernetesVersion",
        "required": "true",
        "type": "string",
        "default": k8sversionlist[0]
        },
        {
        "variable": "rancherKubernetesEngineConfig.network.plugin",
        "required": "true",
        "type": "string",
        "default": "canal"
        },
        {
        "variable": "rancherKubernetesEngineConfig.services.etcd.backupConfig."
                    "s3BackupConfig.bucketName",
        "required": "true",
        "type": "string",
        "default": ""
        },
        {
        "variable": "rancherKubernetesEngineConfig.services.etcd.backupConfig."
                    "s3BackupConfig.endpoint",
        "required": "true",
        "type": "string",
        "default": ""

        },
        {
        "variable": "rancherKubernetesEngineConfig.services.etcd.backupConfig."
                    "s3BackupConfig.accessKey",
        "required": "true",
        "type": "string",
        "default": ""
        },
        {
        "variable": "rancherKubernetesEngineConfig.services.etcd.backupConfig."
                    "s3BackupConfig.secretKey",
        "required": "true",
        "type": "string",
        "default": ""
        }]

    answers = {
        "values": {
            "rancherKubernetesEngineConfig.kubernetesVersion":
                k8sversionlist[1],
            "rancherKubernetesEngineConfig.network.plugin": "flannel",
            "rancherKubernetesEngineConfig.services.etcd.backupConfig."
                "s3BackupConfig.bucketName": RANCHER_S3_BUCKETNAME,
            "rancherKubernetesEngineConfig.services.etcd.backupConfig."
                "s3BackupConfig.endpoint": RANCHER_S3_ENDPOINT,
            "rancherKubernetesEngineConfig.services.etcd.backupConfig."
                "s3BackupConfig.accessKey": AWS_ACCESS_KEY_ID,
            "rancherKubernetesEngineConfig.services.etcd.backupConfig."
                "s3BackupConfig.secretKey": AWS_SECRET_ACCESS_KEY
        }
    }

    standard_user_client = \
        get_client_for_token(user_token["user_standard"]["token"])
    cluster_template = standard_user_client.create_cluster_template(
            name=random_test_name("template"), description="test-template")
    clusterTemplateId = cluster_template.id

    cluster_template_revision = \
        standard_user_client.create_cluster_template_revision(
            name=random_test_name("revision"),
            clusterConfig=cluster_config,
            clusterTemplateId=clusterTemplateId,
            enabled="true", questions=questions)
    cluster = create_node_cluster(
                standard_user_client, name=random_test_name("test-auto"),
                clusterTemplateRevisionId=cluster_template_revision.id,
                answers=answers)

    # Verify that the cluster's applied spec has the parameters set as expected
    assert cluster.appliedSpec.dockerRootDir == "/var/lib/docker123"
    assert cluster.appliedSpec.localClusterAuthEndpoint.enabled is True
    assert cluster.appliedSpec.rancherKubernetesEngineConfig.\
        kubernetesVersion == k8sversionlist[1]
    assert cluster.appliedSpec.rancherKubernetesEngineConfig.services.etcd.\
        backupConfig.s3BackupConfig.bucketName == RANCHER_S3_BUCKETNAME
    assert cluster.appliedSpec.rancherKubernetesEngineConfig.services.\
        etcd.backupConfig.s3BackupConfig.endpoint == RANCHER_S3_ENDPOINT
    assert cluster.appliedSpec.rancherKubernetesEngineConfig.services.etcd.\
        backupConfig.s3BackupConfig.accessKey == AWS_ACCESS_KEY_ID
    assert cluster.appliedSpec.rancherKubernetesEngineConfig.services.etcd.\
        backupConfig.s3BackupConfig.type == "/v3/schemas/s3BackupConfig"
    assert cluster.appliedSpec.rancherKubernetesEngineConfig.network.plugin ==\
        "flannel"

    check_cluster_version(cluster, k8sversionlist[1])

    # Verify flannel pod in the kube-system namespace
    cmd = "get pods -l k8s-app=flannel --namespace kube-system"
    pod_result = execute_kubectl_cmd(cmd)

    assert (len(["items"])) == 1

    for pod in pod_result["items"]:
        print(pod["metadata"]["name"])
        assert "flannel" in (pod["metadata"]["name"])

    # Perform Backup
    backup = cluster.backupEtcd()
    backupname = backup['metadata']['name']
    etcdbackups = cluster.etcdBackups(name=backupname)
    etcdbackupdata = etcdbackups['data']
    s3backupconfig = etcdbackupdata[0]['backupConfig']['s3BackupConfig']
    assert s3backupconfig['type'] == '/v3/schemas/s3BackupConfig'
    backupId = etcdbackupdata[0]['id']
    print("BackupId", backupId)
    wait_for_backup_to_active(standard_user_client, cluster, backupname)

    cluster_cleanup(standard_user_client, cluster)


def test_cluster_template_create_edit_adminuser():

    # Create an admin client . As an admin, create a RKE template and
    # revisions R1 and R2. Create a cluster using R1.
    # Edit and change revision to R2

    client = get_admin_client()
    cluster_template_create_edit(client)


def test_cluster_template_create_edit_stduser():
    # Create a standard user client . As a standard user, create a RKE
    # template and revisions R1 and R2. Create a cluster using R1.
    # Edit and change revision to R2

    standard_user_client = \
        get_client_for_token(user_token["user_standard"]["token"])
    cluster_template_create_edit(standard_user_client)


def test_cluster_template_add_owner():

    # This test case tests the owner member role of the cluster template
    k8sversionlist = get_k8s_versionlist()
    cluster_config1 = get_cluster_config(k8sversionlist[0])
    cluster_config2 = get_cluster_config(k8sversionlist[1])
    client = get_admin_client()

    standard_newuser_client = \
        get_client_for_token(user_token["newuser_standard"]["token"])
    # As an Admin, create a cluster template, revision and update the members
    # list with the new user as owner
    template_name = random_test_name("template")
    cluster_template = client.create_cluster_template(
        name=template_name, description="test-template")

    principalid = user_token["newuser_standard"]["user"]["principalIds"]
    members = [{
        "type": "member",
        "accessType": "owner",
        "userPrincipalId": principalid
    }]

    cluster_template = client.update(cluster_template,
                                     name=template_name,
                                     members=members)
    # As an owner of the template, create a revision using the template
    # and also create a cluster using the template revision
    revision_name = random_test_name("revision1")
    cluster_template_revision = \
        standard_newuser_client.create_cluster_template_revision(
            name=revision_name,
            clusterConfig=cluster_config1,
            clusterTemplateId=cluster_template.id)
    cluster = create_node_cluster(
        standard_newuser_client, name=random_test_name("test-auto"),
        clusterTemplateRevisionId=cluster_template_revision.id)

    # As an admin, create another template and a revision.
    cluster_template_new = client.create_cluster_template(
        name="new_template", description="newtest-template")
    revision_name = random_test_name("revision2")
    cluster_template_newrevision = \
        client.create_cluster_template_revision(
            name=revision_name,
            clusterConfig=cluster_config2,
            clusterTemplateId=cluster_template_new.id)

    # Verify that the existing standard user cannot create a new revision using
    #  this template
    with pytest.raises(ApiError) as e:
        standard_newuser_client.create_cluster_template_revision(
            name=random_test_name("userrevision"),
            clusterConfig=cluster_config2,
            clusterTemplateId=cluster_template_new.id)

    print(e.value.error.status)
    print(e.value.error.code)
    assert e.value.error.status == 404
    assert e.value.error.code == "NotFound"

    # Verify that the existing standard user cannot create a cluster
    # using the new revision
    with pytest.raises(ApiError) as e:
        create_node_cluster(
            standard_newuser_client, name=random_test_name("test-auto"),
            clusterTemplateRevisionId=cluster_template_newrevision.id)
    print(e)
    assert e.value.error.status == 404
    assert e.value.error.code == "NotFound"

    cluster_cleanup(standard_newuser_client, cluster)


def test_cluster_template_add_readonly_member():

    # This test case tests a read-only member role of the cluster template
    k8sversionlist = get_k8s_versionlist()
    cluster_config1 = get_cluster_config(k8sversionlist[0])
    client = get_admin_client()

    standard_newuser_client = \
        get_client_for_token(user_token["newuser_standard"]["token"])
    # As an Admin, create a cluster template, revision and update the members
    # list with the new user as read-only user
    template_name = random_test_name("usertemplate")
    cluster_template = client.create_cluster_template(
        name=template_name, description="test-template")

    revision_name = random_test_name("revision1")
    cluster_template_revision1 = client.create_cluster_template_revision(
        name=revision_name,
        clusterConfig=cluster_config1,
        clusterTemplateId=cluster_template.id)

    principalid = user_token["newuser_standard"]["user"]["principalIds"]
    members = [{
        "type": "member",
        "accessType": "read-only",
        "userPrincipalId": principalid
    }]

    cluster_template = client.update(cluster_template,
                                     name=template_name, members=members)

    # As a read-only member of the rke template, verify that
    # adding another revision to the template fails
    revision_name = "userrevision"
    with pytest.raises(ApiError) as e:
        standard_newuser_client.create_cluster_template_revision(
            name=revision_name,
            clusterConfig=cluster_config1,
            clusterTemplateId=cluster_template.id)

    assert e.value.error.status == 403
    assert e.value.error.code == 'PermissionDenied'

    # Verify that the read-only user can create a cluster with the existing
    # template revision
    cluster = create_node_cluster(
        standard_newuser_client, name=random_test_name("test-auto"),
        clusterTemplateRevisionId=cluster_template_revision1.id)

    # As an admin, create another template and a revision.
    cluster_template_new = client.create_cluster_template(
        name="new_template", description="newtest-template")
    revision_name = random_test_name("revision2")
    cluster_template_newrevision = \
        client.create_cluster_template_revision(
            name=revision_name,
            clusterConfig=cluster_config1,
            clusterTemplateId=cluster_template_new.id)

    # Verify that the existing standard user cannot create a cluster
    # using the new revision
    with pytest.raises(ApiError) as e:
        create_node_cluster(
            standard_newuser_client, name=random_test_name("test-auto"),
            clusterTemplateRevisionId=cluster_template_newrevision.id)
    print(e)
    assert e.value.error.status == 404
    assert e.value.error.code == "NotFound"

    cluster_cleanup(standard_newuser_client, cluster)


def cluster_template_create_edit(testclient):

    # Method to create cluster template revisions R1, R2.
    # Create a cluster with a RKE template revision R1.
    # Then edit the cluster and change the revision to R2

    k8sversionlist = get_k8s_versionlist()
    cluster_config1 = get_cluster_config(k8sversionlist[0])
    cluster_config2 = get_cluster_config(k8sversionlist[1])

    client = testclient

    cluster_template = client.create_cluster_template(
        name=random_test_name("template"), description="test-template")

    cluster_template_revision1 = client.create_cluster_template_revision(
        name=random_test_name("revision1"),
        clusterConfig=cluster_config1,
        clusterTemplateId=cluster_template.id)

    cluster_name = random_test_name("test-auto")
    cluster = create_node_cluster(
        client, name=cluster_name,
        clusterTemplateRevisionId=cluster_template_revision1.id)
    check_cluster_version(cluster, k8sversionlist[0])

    cluster_template_revision2 = client.create_cluster_template_revision(
        name=random_test_name("revision2"),
        clusterConfig=cluster_config2,
        clusterTemplateId=cluster_template.id)

    cluster = client.update(
            cluster,
            name=cluster_name,
            clusterTemplateRevisionId=cluster_template_revision2.id)

    cluster = validate_cluster(client,
                               cluster,
                               intermediate_state="updating")
    check_cluster_version(cluster, k8sversionlist[1])
    cluster_cleanup(client, cluster)


def node_template_do():
    client = get_admin_client()
    do_cloud_credential_config = {"accessToken": DO_ACCESSKEY}
    do_cloud_credential = client.create_cloud_credential(
        digitaloceancredentialConfig=do_cloud_credential_config
    )
    node_template = client.create_node_template(
        digitaloceanConfig={"region": "nyc3",
                            "size": "4gb",
                            "image": "ubuntu-16-04-x64"},
        name=random_name(),
        driver="digitalocean",
        namespaceId="dig",
        cloudCredentialId=do_cloud_credential.id,
        useInternalIpAddress=True)
    node_template = client.wait_success(node_template)
    return node_template


def create_node_cluster(client, name, clusterTemplateRevisionId, answers=None):

    cluster = client.create_cluster(
        name=name,
        clusterTemplateRevisionId=clusterTemplateRevisionId,
        answers=answers)
    nodetemplate = node_template_do()
    nodes = []
    node = {"hostnamePrefix": random_test_name("test-auto"),
            "nodeTemplateId": nodetemplate.id,
            "requestedHostname": "test-auto-template",
            "controlPlane": True,
            "etcd": True,
            "worker": True,
            "quantity": 1,
            "clusterId": None}
    nodes.append(node)
    node_pools = []
    for node in nodes:
        node["clusterId"] = cluster.id
        node_pool = client.create_node_pool(**node)
        node_pool = client.wait_success(node_pool)
        node_pools.append(node_pool)

    cluster = validate_cluster(client, cluster)
    nodes = client.list_node(clusterId=cluster.id).data
    assert len(nodes) == len(nodes)
    for node in nodes:
        assert node.state == "active"

    return cluster


def getRKEConfig(k8sversion):
    rke_config = {
        "addonJobTimeout": 30,
        "ignoreDockerVersion": "true",
        "sshAgentAuth": "false",
        "type": "rancherKubernetesEngineConfig",
        "kubernetesVersion": k8sversion,
        "authentication": {
            "strategy": "x509",
            "type": "authnConfig"
        },
        "network": {
            "plugin": "canal",
            "type": "networkConfig",
            "options": {
                "flannel_backend_type": "vxlan"
            }
        },
        "ingress": {
            "provider": "nginx",
            "type": "ingressConfig"
        },
        "monitoring": {
            "provider": "metrics-server",
            "type": "monitoringConfig"
        },
        "services": {
            "type": "rkeConfigServices",
            "kubeApi": {
                "alwaysPullImages": "false",
                "podSecurityPolicy": "false",
                "serviceNodePortRange": "30000-32767",
                "type": "kubeAPIService"
            },
            "etcd": {
                "creation": "12h",
                "extraArgs": {
                    "heartbeat-interval": 500,
                    "election-timeout": 5000
                },
                "retention": "72h",
                "snapshot": "false",
                "type": "etcdService",
                "backupConfig": {
                    "enabled": "true",
                    "intervalHours": 12,
                    "retention": 6,
                    "type": "backupConfig",
                    "s3BackupConfig": {
                      "type": "s3BackupConfig",
                      "accessKey": AWS_ACCESS_KEY_ID,
                      "secretKey": AWS_SECRET_ACCESS_KEY,
                      "bucketName": "test-auto-s3",
                      "endpoint": "s3.amazonaws.com"
                    }
                }
            }
        }
    }
    return rke_config
