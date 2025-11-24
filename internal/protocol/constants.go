package protocol

// MagicSignal is sent by the server through the GET response tunnel
// to inform the frpc client to dial a connection to the local port.
// This signal is sent when a new TCP connection arrives at the server.
var MagicSignal = []byte{0xFF, 0xFE, 0xFD, 0xFC, 0xFB, 0xFA, 0xF9, 0xF8}