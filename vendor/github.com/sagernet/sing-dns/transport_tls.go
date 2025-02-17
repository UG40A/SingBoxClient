package dns

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"net"
	"net/url"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"github.com/miekg/dns"
)

var _ Transport = (*TLSTransport)(nil)

func init() {
	RegisterTransport([]string{"tls"}, CreateTLSTransport)
}

func CreateTLSTransport(ctx context.Context, logger logger.ContextLogger, dialer N.Dialer, link string) (Transport, error) {
	serverURL, err := url.Parse(link)
	if err != nil {
		return nil, err
	}
	return NewTLSTransport(ctx, dialer, M.ParseSocksaddr(serverURL.Host))
}

type TLSTransport struct {
	myTransportAdapter
}

func NewTLSTransport(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr) (*TLSTransport, error) {
	if !serverAddr.IsValid() {
		return nil, E.New("invalid server address")
	}
	if serverAddr.Port == 0 {
		serverAddr.Port = 853
	}
	transport := &TLSTransport{
		newAdapter(ctx, dialer, serverAddr),
	}
	transport.handler = transport
	return transport, nil
}

func (t *TLSTransport) DialContext(ctx context.Context, queryCtx context.Context) (net.Conn, error) {
	conn, err := t.dialer.DialContext(ctx, N.NetworkTCP, t.serverAddr)
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(conn, &tls.Config{
		ServerName: t.serverAddr.AddrString(),
	})
	err = tlsConn.HandshakeContext(queryCtx)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return tlsConn, nil
}

func (t *TLSTransport) ReadMessage(conn net.Conn) (*dns.Msg, error) {
	var length uint16
	err := binary.Read(conn, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}
	_buffer := buf.StackNewSize(int(length))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	_, err = buffer.ReadFullFrom(conn, int(length))
	if err != nil {
		return nil, err
	}
	var message dns.Msg
	err = message.Unpack(buffer.Bytes())
	return &message, err
}

func (t *TLSTransport) WriteMessage(conn net.Conn, message *dns.Msg) error {
	rawMessage, err := message.Pack()
	if err != nil {
		return err
	}
	_buffer := buf.StackNewSize(2 + len(rawMessage))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(binary.Write(buffer, binary.BigEndian, uint16(len(rawMessage))))
	common.Must1(buffer.Write(rawMessage))
	return common.Error(conn.Write(buffer.Bytes()))
}
