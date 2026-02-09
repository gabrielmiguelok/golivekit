// Package router provides HTTP routing for GoliveKit applications.
package router

import (
	"github.com/gabrielmiguelok/golivekit/pkg/core"
	"github.com/gabrielmiguelok/golivekit/pkg/protocol"
	"github.com/gabrielmiguelok/golivekit/pkg/transport"
)

// TransportAdapter adapta transport.WebSocketTransport a core.Transport.
// Permite que el Socket del core use el transport de WebSocket.
type TransportAdapter struct {
	ws    *transport.WebSocketTransport
	codec protocol.Codec
}

// NewTransportAdapter crea un nuevo adaptador de transporte.
func NewTransportAdapter(ws *transport.WebSocketTransport, codec protocol.Codec) *TransportAdapter {
	if codec == nil {
		codec = protocol.NewPhoenixCodec()
	}
	return &TransportAdapter{
		ws:    ws,
		codec: codec,
	}
}

// Send envía un mensaje a través del WebSocket.
// Implementa core.Transport.
func (a *TransportAdapter) Send(msg core.Message) error {
	// Convertir core.Message a transport.Message
	transportMsg := transport.Message{
		Ref:     msg.Ref,
		Topic:   msg.Topic,
		Event:   msg.Event,
		Payload: msg.Payload,
	}

	return a.ws.Send(transportMsg)
}

// Close cierra el transporte.
// Implementa core.Transport.
func (a *TransportAdapter) Close() error {
	return a.ws.Close()
}

// IsConnected retorna si el transporte está conectado.
// Implementa core.Transport.
func (a *TransportAdapter) IsConnected() bool {
	return a.ws.IsConnected()
}

// WebSocket retorna el transporte WebSocket subyacente.
func (a *TransportAdapter) WebSocket() *transport.WebSocketTransport {
	return a.ws
}

// Codec retorna el codec usado para serialización.
func (a *TransportAdapter) Codec() protocol.Codec {
	return a.codec
}

// ProtocolTransportAdapter adapta el transporte para usar protocol.Message.
// Proporciona métodos adicionales para trabajar con el protocolo Phoenix.
type ProtocolTransportAdapter struct {
	*TransportAdapter
}

// NewProtocolTransportAdapter crea un adaptador con soporte completo de protocolo.
func NewProtocolTransportAdapter(ws *transport.WebSocketTransport, codec protocol.Codec) *ProtocolTransportAdapter {
	return &ProtocolTransportAdapter{
		TransportAdapter: NewTransportAdapter(ws, codec),
	}
}

// SendProtocolMessage envía un mensaje usando el protocolo completo.
func (a *ProtocolTransportAdapter) SendProtocolMessage(msg *protocol.Message) error {
	data, err := a.codec.Encode(msg)
	if err != nil {
		return err
	}

	// Crear transport.Message desde los bytes codificados
	transportMsg := transport.Message{
		Ref:     msg.Ref,
		Topic:   msg.Topic,
		Event:   msg.Event,
		Payload: msg.Payload,
	}

	// Para Phoenix codec, enviamos directamente el array JSON
	if a.codec.Name() == "phoenix" {
		// El codec Phoenix ya produce el formato correcto
		return a.sendRaw(data)
	}

	return a.ws.Send(transportMsg)
}

// sendRaw envía bytes crudos por el WebSocket.
func (a *ProtocolTransportAdapter) sendRaw(data []byte) error {
	// Necesitamos enviar los datos como string en el payload
	// ya que transport.Message hace Marshal interno
	msg := transport.Message{
		Payload: map[string]any{"_raw": string(data)},
	}
	return a.ws.Send(msg)
}

// SendReply envía una respuesta al cliente.
func (a *ProtocolTransportAdapter) SendReply(ref, topic string, status string, response map[string]any) error {
	msg := protocol.ReplyMessage(ref, topic, status, response)
	return a.SendProtocolMessage(msg)
}

// SendOk envía una respuesta exitosa.
func (a *ProtocolTransportAdapter) SendOk(ref, topic string, response map[string]any) error {
	return a.SendReply(ref, topic, "ok", response)
}

// SendError envía una respuesta de error.
func (a *ProtocolTransportAdapter) SendError(ref, topic string, reason string) error {
	msg := protocol.ErrorReply(ref, topic, reason)
	return a.SendProtocolMessage(msg)
}

// SendDiff envía un diff al cliente.
func (a *ProtocolTransportAdapter) SendDiff(topic string, diff map[string]any) error {
	msg := protocol.DiffMessage(topic, diff)
	return a.SendProtocolMessage(msg)
}

// SendBroadcast envía un mensaje broadcast.
func (a *ProtocolTransportAdapter) SendBroadcast(topic, event string, payload map[string]any) error {
	msg := protocol.BroadcastMessage(topic, event, payload)
	return a.SendProtocolMessage(msg)
}
