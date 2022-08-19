package nodes

import (
	"os"
	"os/user"
	"path/filepath"

	"github.com/rancher/rancher/tests/framework/pkg/config"
	"golang.org/x/crypto/ssh"
)

const (
	// The json/yaml config key for the config of nodes of outside cloud provider e.g. linode or ec2
	ExternalNodeConfigConfigurationFileKey = "externalNodes"
	SSHPathConfigurationKey                = "sshPath"
	defaultSSHPath                         = ".ssh"
)

// SSHPath is the path to the ssh key used in external node functionality. This be used if the ssh keys exists
// in a location not in /.ssh
type SSHPath struct {
	SSHPath string `json:"sshPath" yaml:"sshPath"`
}

// Node is a configuration of node that is from an oudise cloud provider
type Node struct {
	NodeID          string `json:"nodeID" yaml:"nodeID"`
	PublicIPAddress string `json:"publicIPAddress" yaml:"publicIPAddress"`
	SSHUser         string `json:"sshUser" yaml:"sshUser"`
	SSHKeyName      string `json:"sshKeyName" yaml:"sshKeyName"`
	SSHKey          []byte
}

// ExternalNodeConfig is a struct that is a collection of the node configurations
type ExternalNodeConfig struct {
	Nodes map[int][]*Node `json:"nodes" yaml:"nodes"`
}

// ExecuteCommand executes `command` in the specific node created.
func (n *Node) ExecuteCommand(command string) (string, error) {
	signer, err := ssh.ParsePrivateKey(n.SSHKey)
	var output []byte
	var output_string string

	if err != nil {
		return output_string, err
	}

	auths := []ssh.AuthMethod{ssh.PublicKeys([]ssh.Signer{signer}...)}

	cfg := &ssh.ClientConfig{
		User:            n.SSHUser,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	cfg.SetDefaults()

	client, err := ssh.Dial("tcp", n.PublicIPAddress+":22", cfg)
	if err != nil {
		return output_string, err
	}

	session, err := client.NewSession()
	if err != nil {
		return output_string, err
	}

	output, err = session.Output(command)
	output_string = string(output)
	return output_string, err
}

// GetSSHKey reads in the ssh file from the .ssh directory, returns the key in []byte format
func GetSSHKey(sshKeyname string) ([]byte, error) {
	var keyPath string

	sshPathConfig := new(SSHPath)

	config.LoadConfig(SSHPathConfigurationKey, sshPathConfig)
	if sshPathConfig.SSHPath == "" {
		user, err := user.Current()
		if err != nil {
			return nil, err
		}

		keyPath = filepath.Join(user.HomeDir, defaultSSHPath, sshKeyname)
	} else {
		keyPath = filepath.Join(sshPathConfig.SSHPath, sshKeyname)
	}
	content, err := os.ReadFile(keyPath)
	if err != nil {
		return []byte{}, err
	}

	return content, nil
}
