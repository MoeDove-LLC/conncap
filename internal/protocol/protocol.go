package protocol

const (
	MsgHello    = "HELLO"
	MsgOK       = "OK"
	MsgPing     = "PING"
	MsgPong     = "PONG"
	MsgRegister = "REGISTER"

	Delimiter = '\n'
)

const (
	HelloTimeout    = 10 // seconds
	DefaultPort     = 8888
	DefaultTCPPort  = 8888
	DefaultUDPPort  = 8888
	DefaultStatsPort = 0
)
