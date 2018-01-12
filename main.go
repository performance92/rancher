package main

import (
	"context"

	"os"

	"github.com/docker/docker/pkg/reexec"
	"github.com/rancher/norman/signal"
	"github.com/rancher/rancher/app"
	"github.com/rancher/rancher/k8s"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

type Config struct {
	KubeConfig        string
	HTTPListenPort    int
	InteralListenPort int
	K8sMode           string
	AddLocal          bool
	Debug             bool
}

func main() {
	if reexec.Init() {
		return
	}

	os.Unsetenv("SSH_AUTH_SOCK")
	os.Unsetenv("SSH_AGENT_PID")

	config := Config{}

	app := cli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "kubeconfig",
			Usage:       "Kube config for accessing k8s cluster",
			EnvVar:      "KUBECONFIG",
			Destination: &config.KubeConfig,
		},
		cli.BoolFlag{
			Name:        "add-local",
			Usage:       "Add local cluster to management server",
			Destination: &config.AddLocal,
		},
		cli.BoolFlag{
			Name:        "debug",
			Usage:       "Enable debug logs",
			Destination: &config.Debug,
		},
		cli.IntFlag{
			Name:        "http-listen-port",
			Usage:       "HTTP listen port",
			Value:       8080,
			Destination: &config.HTTPListenPort,
		},
		cli.IntFlag{
			Name:        "internal-api-listen-port",
			Usage:       "Listen port to embedded k8s API server",
			Value:       8081,
			Destination: &config.InteralListenPort,
		},
		cli.StringFlag{
			Name:        "k8s-mode",
			Usage:       "Mode to run or access k8s API server for management API (internal, exec)",
			Value:       "internal",
			Destination: &config.K8sMode,
		},
	}

	app.Action = func(c *cli.Context) error {
		return run(config)
	}

	app.ExitErrHandler = func(c *cli.Context, err error) {
		logrus.Fatal(err)
	}

	app.Run(os.Args)
}

func run(cfg Config) error {
	if cfg.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	ctx := signal.SigTermCancelContext(context.Background())

	os.Args = []string{os.Args[0]}
	kubeConfig, local, err := k8s.GetConfig(ctx, cfg.K8sMode, cfg.AddLocal, cfg.KubeConfig, cfg.InteralListenPort)
	if err != nil {
		return err
	}

	return app.Run(ctx, *kubeConfig, cfg.HTTPListenPort, local)
}
