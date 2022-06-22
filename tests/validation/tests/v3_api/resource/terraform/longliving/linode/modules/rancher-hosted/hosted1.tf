provider "helm" {
  alias = "rancher-hosted1"
  kubernetes {
    config_path = local_file.kube_config_hosted1_yaml.filename
  }
}

provider "rancher2" {
  alias     = "bootstrap_hosted1"
  api_url   = "https://${var.hosted1_load_balancer_subdomain}.${var.load_balancers_domain}"
  insecure  = true
  bootstrap = true
}

provider "rancher2" {
  alias     = "admin_hosted1"
  api_url   = rancher2_bootstrap.admin_hosted1.url
  insecure  = true
  token_key = rancher2_bootstrap.admin_hosted1.token
  timeout   = "10m"
}

resource "rancher2_auth_config_github" "github_hosted1" {
  provider = rancher2.admin_hosted1
  client_id     = var.rancher_github_client_id_hosted1
  client_secret = var.rancher_github_client_secret_hosted1
  enabled       = true
}

resource "aws_route53_record" "hosted1_rancher" {
  zone_id = var.zone_id
  name    = "${var.hosted1_load_balancer_subdomain}.${var.load_balancers_domain}"
  type    = "A"
  ttl     = "10"
  records = [linode_instance.hosted1_lb.ip_address]
  depends_on = [linode_instance.hosted1_lb]
}

resource "ssh_resource" "retrieve_config_hosted1" {
  host = linode_instance.hosted1_node1.ip_address
  commands = [
    "sed \"s/127.0.0.1/${linode_instance.hosted1_node1.ip_address}/g\" /etc/rancher/k3s/k3s.yaml"
  ]
  user  = "root"
  agent = false
  private_key = base64decode("${var.ssh_private_key}")
  depends_on = [
    linode_instance.hosted1_node1,
    ssh_resource.retrieve_config_super
    ]
}

resource "ssh_resource" "retrieve_token_hosted1" {
  host = linode_instance.hosted1_node1.ip_address
  commands = [
    "cat /var/lib/rancher/k3s/server/node-token"
  ]
  user  = "root"
  agent = false
  private_key = base64decode("${var.ssh_private_key}")

  depends_on = [
    linode_instance.hosted1_node1,
    ssh_resource.retrieve_token
    ]
}

resource "local_file" "kube_config_hosted1_yaml" {
  filename = format("%s/%s", path.root, "kube_config_server_hosted1.yaml")
  content  = ssh_resource.retrieve_config_hosted1.result
}

resource "local_file" "k3s_token_hosted1" {
  filename = format("%s/%s", path.root, "k3s_token_hosted1")
  content  = ssh_resource.retrieve_token_hosted1.result
}

resource "linode_instance" "hosted1_lb" {
    label = "${var.linode_resource_prefix}hosted1lb-longliving"
    image = "linode/ubuntu20.04"
    region = "us-east"
    type = "g6-standard-2"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_hosted1"
    tags = [ "hosted_hosted1" ]
    swap_size = 256
    private_ip = true
    
    alerts {
    cpu            = 0
    io             = 0
    network_in     = 0
    network_out    = 0
    transfer_quota = 0
  }

  connection {
      host = self.ip_address
      user = "root"
      password = var.linode_root_password
  }

  provisioner "file" {
    source      = "${path.module}/scripts/certs"
    destination = "certs"
  }
 
  provisioner "file" {
    source      = "${path.module}/scripts/nginx"
    destination = "nginx"
  }
  provisioner "remote-exec" {
    inline = [
        "hostnamectl set-hostname ${var.linode_resource_prefix}hosted1lb",
        "wget https://releases.rancher.com/install-docker/${var.docker_version}.sh",
        "chmod +x ${var.docker_version}.sh",
        "bash ${var.docker_version}.sh",
        "sed -i \"s/<host1>/${linode_instance.hosted1_node1.ip_address}/g\" nginx/nginx.conf",
        "sed -i \"s/<host2>/${linode_instance.hosted1_node2.ip_address}/g\" nginx/nginx.conf",
        "sed -i \"s/<host3>/${linode_instance.hosted1_node3.ip_address}/g\" nginx/nginx.conf",
        "sed -i \"s/<FQDN>/${var.hosted1_load_balancer_subdomain}.${var.load_balancers_domain}/g\" nginx/nginx.conf",
        "docker run --name docker-nginx -p 80:80 -p 443:443 -v $(pwd)/certs/:/certs/ -v $(pwd)/nginx/nginx.conf:/etc/nginx/nginx.conf -d nginx"
    ]
  }

  depends_on = [
    local_file.fullchain,
    local_file.privkey
  ]
}

resource "linode_instance" "hosted1_node1" {
    label = "${var.linode_resource_prefix}hosted1n1-longliving"
    image = "linode/ubuntu20.04"
    region = "us-east"
    type = "g6-dedicated-4"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_hosted1"
    tags = [ "hosted_hosted1" ]
    swap_size = 256
    private_ip = true

    alerts {
      cpu            = 0
      io             = 0
      network_in     = 0
      network_out    = 0
      transfer_quota = 0
    }

    connection {
      host = self.ip_address
      user = "root"
      password = var.linode_root_password
    }

    provisioner "remote-exec" {
      inline = [
        "hostnamectl set-hostname ${var.linode_resource_prefix}hosted1n1",
        "echo \"vm.panic_on_oom=0\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"vm.overcommit_memory=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic=10\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic_on_oops=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "sysctl -p /etc/sysctl.d/90-kubelet.conf",
        "systemctl restart systemd-sysctl",
        "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--disable=traefik' INSTALL_K3S_VERSION='${var.k3s_version_hosted1}' K3S_TOKEN=${random_string.k3s_token.result} sh -s - server --node-name ${self.label} --cluster-init --node-external-ip=${self.ip_address} --tls-san ${var.hosted1_load_balancer_subdomain}.${var.load_balancers_domain}"
      ]
    }

    provisioner "file" {
      source      = "${path.module}/manifests/ingress-nginx.yaml"
      destination = "/var/lib/rancher/k3s/server/manifests/ingress-nginx.yaml"
    }
}

resource "linode_instance" "hosted1_node2" {
    label = "${var.linode_resource_prefix}hosted1n2-longliving"
    image = "linode/ubuntu20.04"
    region = "us-east"
    type = "g6-dedicated-4"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_hosted1"
    tags = [ "hosted_hosted1" ]
    swap_size = 256
    private_ip = true

    alerts {
      cpu            = 0
      io             = 0
      network_in     = 0
      network_out    = 0
      transfer_quota = 0
    }

    connection {
      host = self.ip_address
      user = "root"
      password = var.linode_root_password
    }

    provisioner "file" {
      source      = format("%s/%s", path.root, "k3s_token_hosted1")
      destination = "k3s_token"
    }
    
    provisioner "remote-exec" {
      inline = [
        "hostnamectl set-hostname ${var.linode_resource_prefix}hosted1n2",
        "echo \"vm.panic_on_oom=0\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"vm.overcommit_memory=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic=10\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic_on_oops=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "sysctl -p /etc/sysctl.d/90-kubelet.conf",
        "systemctl restart systemd-sysctl",
        "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--disable=traefik' INSTALL_K3S_VERSION='${var.k3s_version_hosted1}' K3S_TOKEN=`cat k3s_token` sh -s - server --node-name ${self.label} --server https://${linode_instance.hosted1_node1.ip_address}:6443 --node-external-ip=${self.ip_address} --tls-san ${var.hosted1_load_balancer_subdomain}.${var.load_balancers_domain}"
      ]
    }

    depends_on = [local_file.k3s_token_hosted1]
}

resource "linode_instance" "hosted1_node3" {
    label = "${var.linode_resource_prefix}hosted1n3-longliving"
    image = "linode/ubuntu20.04"
    region = "us-east"
    type = "g6-dedicated-4"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_hosted1"
    tags = [ "hosted_hostd1" ]
    swap_size = 256
    private_ip = true

    alerts {
      cpu            = 0
      io             = 0
      network_in     = 0
      network_out    = 0
      transfer_quota = 0
    }

    connection {
      host = self.ip_address
      user = "root"
      password = var.linode_root_password
    }

    provisioner "file" {
      source      = format("%s/%s", path.root, "k3s_token_hosted1")
      destination = "k3s_token"
    }

    provisioner "remote-exec" {
      inline = [
        "hostnamectl set-hostname ${var.linode_resource_prefix}hosted1n3",
        "echo \"vm.panic_on_oom=0\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"vm.overcommit_memory=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic=10\" >>/etc/sysctl.d/90-kubelet.conf",
        "echo \"kernel.panic_on_oops=1\" >>/etc/sysctl.d/90-kubelet.conf",
        "sysctl -p /etc/sysctl.d/90-kubelet.conf",
        "systemctl restart systemd-sysctl",
        "curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC='--disable=traefik' INSTALL_K3S_VERSION='${var.k3s_version_hosted1}' K3S_TOKEN=`cat k3s_token` sh -s - server --node-name ${self.label} --server https://${linode_instance.hosted1_node1.ip_address}:6443 --node-external-ip=${self.ip_address} --tls-san ${var.hosted1_load_balancer_subdomain}.${var.load_balancers_domain}"
      ]
    }

    depends_on = [linode_instance.hosted1_node2]
}

resource "null_resource" "import_hosted_cluster1" {
  provisioner "local-exec" {
    command = "${rancher2_cluster.hosted1.cluster_registration_token.0.insecure_command}"

    environment = {
      KUBECONFIG       = local_file.kube_config_hosted1_yaml.filename
      RANCHER_HOSTNAME = "${var.super_load_balancer_subdomain}.${var.load_balancers_domain}"
    }
  }

  depends_on = [
    rancher2_cluster.hosted1,
    linode_instance.hosted1_node3
  ]
}

resource "helm_release" "rancher_hosted1_server" {
  provider         = helm.rancher-hosted1
  name             = "rancher"
  chart            = "https://releases.rancher.com/server-charts/latest/rancher-${var.rancher_version_hosted1}.tgz"
  namespace        = "cattle-system"
  create_namespace = true
  wait             = true

  set {
    name  = "hostname"
    value = "${var.hosted1_load_balancer_subdomain}.${var.load_balancers_domain}"
  }

   set {
    name  = "tls"
    value = "external"
  }

  set {
    name  = "bootstrapPassword"
    value = "admin"
  }

  depends_on = [
    null_resource.wait_for_ingress_rollout_hosted1
  ]
}

resource "null_resource" "wait_for_hosted1_rancher" {
  provisioner "local-exec" {
    command = <<-EOT
    kubectl -n cattle-system rollout status deploy/rancher
    EOT

    environment = {
      KUBECONFIG       = local_file.kube_config_hosted1_yaml.filename
      RANCHER_HOSTNAME = "${var.hosted1_load_balancer_subdomain}.${var.load_balancers_domain}"
    }
  }
  depends_on = [
    helm_release.rancher_hosted1_server
  ]
}

resource "null_resource" "wait_for_ingress_rollout_hosted1" {
  provisioner "local-exec" {
    command = <<-EOT
    exit_test () {
      if [ $? -eq 0 ]; then
        printf "\n Check completed \n"
      else
        printf "\n There was a failure \n" >&2
        exit 1
      fi
    }
    kubectl wait job -n kube-system helm-install-ingress-nginx --for condition=Complete --timeout=30s; exit_test
    kubectl wait pods -n ingress-nginx -l app.kubernetes.io/name=ingress-nginx --for condition=Ready --timeout=30s; exit_test
    kubectl -n ingress-nginx rollout status ds/ingress-nginx-controller; exit_test
    EOT

    environment = {
      KUBECONFIG       = local_file.kube_config_hosted1_yaml.filename
      RANCHER_HOSTNAME = "${var.hosted1_load_balancer_subdomain}.${var.load_balancers_domain}"
    }
  }
  depends_on = [
    aws_route53_record.hosted1_rancher
  ]
}

resource "rancher2_bootstrap" "admin_hosted1" {
  provider         = rancher2.bootstrap_hosted1
  initial_password = "admin"
  password         = var.rancher_password
  depends_on       = [null_resource.wait_for_hosted1_rancher]
}

resource "rancher2_cluster" "custom_cluster1" {
  provider = rancher2.admin_hosted1
  name = "custom-cluster-hosted1"
  description = "Rancher custom-cluster-hosted1"
  enable_cluster_monitoring = false
  rke_config {
    network {
      plugin = "canal"
    }
  }
}

resource "rancher2_cluster_sync" "custom_cluster1" {
    provider = rancher2.admin_hosted1
    cluster_id =  rancher2_cluster.custom_cluster1.id
    state_confirm = 25
    wait_catalogs = true
    depends_on = [
      linode_instance.custom_nodes1
    ]
}

resource "linode_instance" "custom_nodes1"{
    count  = length(local.node_config)
    label  = "${var.linode_resource_prefix}custom-n${count.index}-longliving" 
    image  = "linode/ubuntu20.04"
    region = "us-east"
    type   = "g6-dedicated-4"
    authorized_keys = ["${var.ssh_authorized_key}"]
    root_pass = var.linode_root_password

    group = "hosted_hosted1"
    tags = [ "hosted_hostd1" ]
    swap_size = 256
    private_ip = true

    alerts {
      cpu            = 0
      io             = 0
      network_in     = 0
      network_out    = 0
      transfer_quota = 0
    }

    connection {
      host = self.ip_address
      user = "root"
      password = var.linode_root_password
    }

    depends_on = [
        rancher2_cluster.custom_cluster1
    ]

    provisioner "remote-exec" {
        inline = [
            "hostnamectl set-hostname ${var.linode_resource_prefix}custom-n${count.index}",
            "wget https://releases.rancher.com/install-docker/${var.docker_version}.sh",
            "chmod +x ${var.docker_version}.sh",
            "bash ${var.docker_version}.sh",
            "${rancher2_cluster.custom_cluster1.cluster_registration_token[0].node_command} --address ${self.ip_address} --internal-address ${self.private_ip_address} --${local.node_config[count.index]}"
        ]
    }

}

resource "rancher2_cloud_credential" "linode_rke2_hosted1" {
  count = local.hosted1_version_ready_for_rke2 ? 1 : 0
  provider = rancher2.admin_hosted1
  name = "linode-rke2-cluster1"
  linode_credential_config {
    token = var.linode_token
  }
}

resource "rancher2_machine_config_v2" "linode_rke2_control_plane_hosted1" {
  count = local.hosted1_version_ready_for_rke2 ? 1 : 0
  provider = rancher2.admin_hosted1
  generate_name = "hosted1-rke2-cp"
  linode_config {
    create_private_ip = true
    image = "linode/ubuntu20.04"
    swap_size = 256
    root_pass = var.linode_root_password
  }
}

resource "rancher2_machine_config_v2" "linode_rke2_etcd_hosted1" {
  count = local.hosted1_version_ready_for_rke2 ? 1 : 0
  provider = rancher2.admin_hosted1
  generate_name = "hosted1-rke2-etcd"
  linode_config {
    create_private_ip = true
    image = "linode/ubuntu20.04"
    swap_size = 256
    root_pass = var.linode_root_password
  }
}

resource "rancher2_machine_config_v2" "linode_rke2_worker_hosted1" {
  count = local.hosted1_version_ready_for_rke2 ? 1 : 0
  provider = rancher2.admin_hosted1
  generate_name = "hosted1-rke2-worker"
  linode_config {
    create_private_ip = true
    image = "linode/ubuntu20.04"
    swap_size = 256
    root_pass = var.linode_root_password
  }
}

resource "rancher2_cluster_v2" "linode_rke2_hosted1" {
  count = local.hosted1_version_ready_for_rke2 ? 1 : 0
  provider = rancher2.admin_hosted1
  name = "longliving-rke2-hosted1"
  kubernetes_version = var.rke2_cluster_version_hosted1
  enable_network_policy = false
  default_cluster_role_for_project_members = "user"
  rke_config {
    machine_pools {
      name = "pool-cp"
      cloud_credential_secret_name = rancher2_cloud_credential.linode_rke2_hosted1[count.index].id
      control_plane_role = true
      etcd_role = false
      worker_role = false
      quantity = 1
      machine_config {
        kind = rancher2_machine_config_v2.linode_rke2_control_plane_hosted1[count.index].kind
        name = rancher2_machine_config_v2.linode_rke2_control_plane_hosted1[count.index].name
      }
    }
    machine_pools {
      name = "pool-etcd"
      cloud_credential_secret_name = rancher2_cloud_credential.linode_rke2_hosted1[count.index].id
      control_plane_role = false
      etcd_role = true
      worker_role = false
      quantity = 1
      machine_config {
        kind = rancher2_machine_config_v2.linode_rke2_etcd_hosted1[count.index].kind
        name = rancher2_machine_config_v2.linode_rke2_etcd_hosted1[count.index].name
      }
    }
    machine_pools {
      name = "pool-worker"
      cloud_credential_secret_name = rancher2_cloud_credential.linode_rke2_hosted1[count.index].id
      control_plane_role = false
      etcd_role = false
      worker_role = true
      quantity = 3
      machine_config {
        kind = rancher2_machine_config_v2.linode_rke2_worker_hosted1[count.index].kind
        name = rancher2_machine_config_v2.linode_rke2_worker_hosted1[count.index].name
      }
    }
  }
}