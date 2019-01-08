package plugin

import plugin "github.com/hashicorp/go-plugin"

// Handshake is configuration information that allows go-plugin clients and servers to perform a handshake.
//
// TODO(ncdc): this should probably be a function so it can't be mutated
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "ARK_PLUGIN",
	MagicCookieValue: "hello",
}
