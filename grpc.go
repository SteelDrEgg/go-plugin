package goplugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	hclog "github.com/hashicorp/go-hclog"
	hcplugin "github.com/hashicorp/go-plugin"
)

func (m *Manager) loadGRPC(ctx context.Context, info Info, pluginRoot string) (backendLoadResult, error) {
	if m.cfg.GRPC == nil {
		return backendLoadResult{}, fmt.Errorf("grpc backend config is not set")
	}
	cfg := m.cfg.GRPC

	commandLine := strings.ReplaceAll(info.Command, "$PLUGIN_ROOT", pluginRoot)
	args, err := splitCommand(commandLine)
	if err != nil {
		return backendLoadResult{}, err
	}
	// The process outlives the startup context after a successful load.
	processCtx, cancelProcess := context.WithCancel(context.Background())
	stopCancellation := context.AfterFunc(ctx, cancelProcess)
	defer stopCancellation()
	owned := false
	defer func() {
		if !owned {
			cancelProcess()
		}
	}()
	cmd := exec.CommandContext(processCtx, args[0], args[1:]...)
	cmd.Dir = pluginRoot
	cmd.Env = []string{"PLUGIN_ROOT=" + pluginRoot}
	if err := withRunAsUser(cmd, cfg.RunAsUser); err != nil {
		return backendLoadResult{}, err
	}

	var exit *exitState
	plugins := defaultGRPCPreset(ctx, cfg)
	clientCfg := &hcplugin.ClientConfig{
		HandshakeConfig:  toHCHandshake(cfg.HandshakeConfig),
		Plugins:          plugins,
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
	for _, plugin := range clientCfg.Plugins {
		if preset, ok := plugin.(*grpcPresetPlugin); ok {
			if exit == nil {
				exit = newExitState()
			}
			preset.observeExit = func(lifetime context.Context) { exit.observeProcess(lifetime, clientCfg.Cmd) }
		}
	}
	dispenseName := resolveDispenseName(clientCfg.Plugins)
	if dispenseName == "" {
		return backendLoadResult{}, fmt.Errorf("grpc preset requires at least one plugin in ClientConfig")
	}

	pluginClient := hcplugin.NewClient(clientCfg)
	cleanup := grpcCleanup(pluginClient, cancelProcess)
	owned = true
	grpcClient, err := pluginClient.Client()
	if err != nil {
		cancelProcess()
		return backendLoadResult{cleanup: cleanup}, fmt.Errorf("connect plugin %q: %w", filepath.Base(commandLine), err)
	}

	raw, err := grpcClient.Dispense(dispenseName)
	if err != nil {
		cancelProcess()
		return backendLoadResult{cleanup: cleanup}, fmt.Errorf("dispense %q: %w", dispenseName, err)
	}

	return backendLoadResult{
		exit:    exit,
		client:  raw,
		cleanup: cleanup,
	}, nil
}

// Hashicorp's Kill blocks and has no context parameter. Run it once so a
// timed-out close can be retried by waiting for the same cleanup operation.
func grpcCleanup(client *hcplugin.Client, cancelProcess context.CancelFunc) func(context.Context) error {
	var once sync.Once
	done := make(chan struct{})
	return func(ctx context.Context) error {
		once.Do(func() {
			go func() {
				defer close(done)
				defer cancelProcess()
				client.Kill()
			}()
		})
		select {
		case <-done:
			return nil
		default:
		}
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			// Force the owned subprocess to exit if graceful shutdown stalls.
			cancelProcess()
			return ctx.Err()
		}
	}
}
