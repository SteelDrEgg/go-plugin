package goplugin

import (
	"context"
	"io"
	"os/exec"

	hcplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

type Config struct {
	TempDir string

	GRPC *GRPCConfig
	WASM *WASMConfig
}

type Protocol string

const (
	ProtocolGRPC Protocol = Protocol(hcplugin.ProtocolGRPC)
)

type HandshakeConfig struct {
	ProtocolVersion  int
	MagicCookieKey   string
	MagicCookieValue string
}

type ClientConfig = hcplugin.ClientConfig

type GRPCConfig struct {
	HandshakeConfig HandshakeConfig

	RunAsUser string

	AllowedProtocols []Protocol
	Stderr           io.Writer
	SyncStdout       io.Writer
	SyncStderr       io.Writer

	// Loader is used by the default gRPC preset.
	// If nil, the preset returns *grpc.ClientConn as client.
	Loader func(ctx context.Context, conn *grpc.ClientConn) (any, error)
	// LoaderWithBroker is used by the default gRPC preset.
	// If set, it takes precedence over Loader and receives GRPCBroker.
	LoaderWithBroker func(ctx context.Context, broker *GRPCBroker, conn *grpc.ClientConn) (any, error)

	ClientConfigOverride func(*ClientConfig)
}

type WASMConfig struct {
	// Loader receives resolved module path from info.Command and returns:
	// 1) plugin client instance used by caller
	// 2) cleanup function invoked on Unload
	Loader func(ctx context.Context, modulePath string, info Info) (client any, cleanup func(context.Context) error, err error)

	// Reserved for future parity with design doc.
	RuntimeConfigOverride any
}

type Info struct {
	Name            string         `yaml:"Name"`
	Version         string         `yaml:"Version"`
	Type            string         `yaml:"Type"`
	ContractVersion int            `yaml:"ContractVersion"`
	Command         string         `yaml:"Command"`
	Metadata        map[string]any `yaml:",inline"`
}

type backendLoadResult struct {
	client  any
	cleanup func(context.Context) error
}

func (c *Config) defaults() {
	if c.GRPC != nil && len(c.GRPC.AllowedProtocols) == 0 {
		c.GRPC.AllowedProtocols = []Protocol{ProtocolGRPC}
	}
}

func (i Info) commandPath() string {
	cmd, _ := splitCommand(i.Command)
	if len(cmd) == 0 {
		return ""
	}
	return cmd[0]
}

func withRunAsUser(cmd *exec.Cmd, username string) error {
	if username == "" {
		return nil
	}
	cred, err := lookupCredential(username)
	if err != nil {
		return err
	}
	cmd.SysProcAttr = cred
	return nil
}
