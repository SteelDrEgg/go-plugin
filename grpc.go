package goplugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	hclog "github.com/hashicorp/go-hclog"
	hcplugin "github.com/hashicorp/go-plugin"
)

func (m *Manager) loadGRPC(_ context.Context, info Info, pluginRoot string) (backendLoadResult, error) {
	if m.cfg.GRPC == nil {
		return backendLoadResult{}, fmt.Errorf("grpc backend config is not set")
	}
	cfg := m.cfg.GRPC

	commandLine := strings.ReplaceAll(info.Command, "$PLUGIN_ROOT", pluginRoot)
	args, err := splitCommand(commandLine)
	if err != nil {
		return backendLoadResult{}, err
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = pluginRoot
	cmd.Env = []string{"PLUGIN_ROOT=" + pluginRoot}
	if err := withRunAsUser(cmd, cfg.RunAsUser); err != nil {
		return backendLoadResult{}, err
	}

	clientCfg := &hcplugin.ClientConfig{
		HandshakeConfig:  toHCHandshake(cfg.HandshakeConfig),
		Plugins:          defaultGRPCPreset(cfg),
		Cmd:              cmd,
		AllowedProtocols: toHCProtocols(cfg.AllowedProtocols),
		SkipHostEnv:      cfg.SkipHostEnv,
		Stderr:           cfg.Stderr,
		SyncStdout:       cfg.SyncStdout,
		SyncStderr:       cfg.SyncStderr,
		Logger: hclog.New(&hclog.LoggerOptions{
			Name:   "go-plugin",
			Output: os.Stderr,
		}),
	}
	if cfg.ClientConfigOverride != nil {
		cfg.ClientConfigOverride(clientCfg)
	}
	dispenseName := resolveDispenseName(clientCfg.Plugins)
	if dispenseName == "" {
		return backendLoadResult{}, fmt.Errorf("grpc preset requires at least one plugin in ClientConfig")
	}

	pluginClient := hcplugin.NewClient(clientCfg)
	grpcClient, err := pluginClient.Client()
	if err != nil {
		pluginClient.Kill()
		return backendLoadResult{}, fmt.Errorf("connect plugin %q: %w", filepath.Base(commandLine), err)
	}

	raw, err := grpcClient.Dispense(dispenseName)
	if err != nil {
		pluginClient.Kill()
		return backendLoadResult{}, fmt.Errorf("dispense %q: %w", dispenseName, err)
	}

	return backendLoadResult{
		client: raw,
		cleanup: func(context.Context) error {
			pluginClient.Kill()
			return nil
		},
	}, nil
}
