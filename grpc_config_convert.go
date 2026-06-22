package goplugin

import hcplugin "github.com/hashicorp/go-plugin"

func toHCHandshake(c HandshakeConfig) hcplugin.HandshakeConfig {
	return hcplugin.HandshakeConfig{
		ProtocolVersion:  uint(c.ProtocolVersion),
		MagicCookieKey:   c.MagicCookieKey,
		MagicCookieValue: c.MagicCookieValue,
	}
}

func toHCProtocols(protocols []Protocol) []hcplugin.Protocol {
	if len(protocols) == 0 {
		return nil
	}
	out := make([]hcplugin.Protocol, 0, len(protocols))
	for _, p := range protocols {
		out = append(out, hcplugin.Protocol(p))
	}
	return out
}
