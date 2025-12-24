package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"cursor2api-go/config"
	"cursor2api-go/middleware"
	"cursor2api-go/models"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

const (
	cursorStreamChatPath = "/aiserver.v1.AiService/StreamChat"
)

// CursorService handles interactions with Cursor IDE API via gRPC-Web.
type CursorService struct {
	config *config.Config
	client *req.Client
}

// NewCursorService creates a new service instance.
func NewCursorService(cfg *config.Config) *CursorService {
	client := req.C()
	client.SetTimeout(time.Duration(cfg.Timeout) * time.Second)
	client.ImpersonateChrome()

	return &CursorService{
		config: cfg,
		client: client,
	}
}

// ChatCompletion creates a chat completion stream for the given request.
func (s *CursorService) ChatCompletion(ctx context.Context, request *models.ChatCompletionRequest) (<-chan interface{}, error) {
	// Validate token
	if s.config.CursorToken == "" {
		return nil, middleware.NewCursorWebError(http.StatusUnauthorized, "CURSOR_TOKEN is not configured")
	}

	// Build protobuf request
	protoData, err := s.buildProtobufRequest(request)
	if err != nil {
		return nil, fmt.Errorf("failed to build protobuf request: %w", err)
	}

	// Build gRPC-Web envelope (5-byte header + protobuf data)
	envelope := s.buildGRPCWebEnvelope(protoData)

	// Generate trace ID
	traceID := uuid.New().String()

	// Build headers
	headers := s.buildHeaders(traceID)

	// Make request
	apiURL := s.config.CursorAPIURL + cursorStreamChatPath
	resp, err := s.client.R().
		SetContext(ctx).
		SetHeaders(headers).
		SetBody(envelope).
		DisableAutoReadResponse().
		Post(apiURL)
	if err != nil {
		return nil, fmt.Errorf("cursor request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Response.Body)
		resp.Response.Body.Close()
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return nil, middleware.NewCursorWebError(resp.StatusCode, message)
	}

	output := make(chan interface{}, 32)
	go s.consumeGRPCWebStream(ctx, resp.Response, output)
	return output, nil
}

// buildProtobufRequest builds a protobuf request from OpenAI format
func (s *CursorService) buildProtobufRequest(request *models.ChatCompletionRequest) ([]byte, error) {
	// Convert OpenAI messages to Cursor protobuf format
	messages := make([]*ChatMessage, 0, len(request.Messages))
	msgUUID := uuid.New().String()

	for _, msg := range request.Messages {
		role := uint64(1) // user
		if msg.Role == "assistant" || msg.Role == "system" {
			role = 2
		}

		content := msg.GetStringContent()
		if s.config.SystemPromptInject != "" && msg.Role == "system" {
			content = content + "\n" + s.config.SystemPromptInject
		}

		messages = append(messages, &ChatMessage{
			Message: content,
			Role:    role,
			Uuid:    msgUUID,
		})
	}

	// Build request
	conversationID := uuid.New().String()
	traceID := uuid.New().String()

	req := &ChatRequest{
		Message:        messages,
		Unknown:        []byte{},
		Paths:          s.config.CursorWorkingDir,
		Model:          &ModelInfo{Model: request.Model, Unknown: []byte{}},
		TraceId:        traceID,
		Unknown1:       0,
		Unknown2:       0,
		ConversationId: conversationID,
		Unknown4:       1,
		Unknown5:       0,
		Unknown6:       0,
		Unknown7:       0,
		Unknown8:       0,
	}

	return req.Marshal()
}

// buildGRPCWebEnvelope builds a gRPC-Web envelope with 5-byte length prefix
func (s *CursorService) buildGRPCWebEnvelope(data []byte) []byte {
	// gRPC-Web format: 1-byte compression flag + 4-byte big-endian length + data
	length := len(data)
	envelope := make([]byte, 5+length)
	envelope[0] = 0 // no compression
	binary.BigEndian.PutUint32(envelope[1:5], uint32(length))
	copy(envelope[5:], data)
	return envelope
}

// buildHeaders builds HTTP headers for the Cursor API request
func (s *CursorService) buildHeaders(traceID string) map[string]string {
	headers := map[string]string{
		"User-Agent":                "connect-es/1.6.1",
		"Authorization":             "Bearer " + s.config.CursorToken,
		"connect-accept-encoding":   "gzip,br",
		"connect-protocol-version":  "1",
		"Content-Type":              "application/grpc-web+proto",
		"x-amzn-trace-id":           "Root=" + traceID,
		"x-cursor-client-version":   s.config.CursorVersion,
		"x-cursor-timezone":         s.config.CursorTimezone,
		"x-ghost-mode":              fmt.Sprintf("%t", s.config.CursorGhostMode),
		"x-request-id":              traceID,
	}

	if s.config.CursorClientKey != "" {
		headers["x-client-key"] = s.config.CursorClientKey
	}

	if s.config.CursorChecksum != "" {
		headers["x-cursor-checksum"] = s.config.CursorChecksum
	}

	return headers
}

// consumeGRPCWebStream reads and parses gRPC-Web stream response
func (s *CursorService) consumeGRPCWebStream(ctx context.Context, resp *http.Response, output chan interface{}) {
	defer close(output)
	defer resp.Body.Close()

	buffer := make([]byte, 0)
	chunk := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := resp.Body.Read(chunk)
		if n > 0 {
			buffer = append(buffer, chunk[:n]...)

			// Parse gRPC-Web chunks from buffer
			for {
				text, consumed, parseErr := s.parseGRPCWebChunk(buffer)
				if parseErr != nil {
					logrus.WithError(parseErr).Debug("Failed to parse gRPC-Web chunk")
					break
				}
				if consumed == 0 {
					break
				}

				buffer = buffer[consumed:]

				if text != "" {
					select {
					case output <- text:
					case <-ctx.Done():
						return
					}
				}
			}
		}

		if err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			if errors.Is(err, context.Canceled) {
				return
			}
			logrus.WithError(err).Error("Error reading gRPC-Web stream")
			errResp := middleware.NewCursorWebError(http.StatusBadGateway, err.Error())
			select {
			case output <- errResp:
			default:
			}
			return
		}
	}
}

// parseGRPCWebChunk parses a single gRPC-Web chunk and extracts text content
func (s *CursorService) parseGRPCWebChunk(buffer []byte) (string, int, error) {
	// gRPC-Web chunk format: delimiter (00 00 00 00) + length info + data
	// Based on the reverse engineering from nekohy/Cursor project

	delimiter := []byte{0x00, 0x00, 0x00, 0x00}
	delimiterIdx := bytes.Index(buffer, delimiter)

	if delimiterIdx == -1 || len(buffer) < delimiterIdx+7 {
		return "", 0, nil // Need more data
	}

	// Check if we have enough bytes after delimiter
	if len(buffer) < delimiterIdx+4+3 {
		return "", 0, nil // Need more data
	}

	byte1 := buffer[delimiterIdx+4]
	byte2 := buffer[delimiterIdx+5]
	byte3 := buffer[delimiterIdx+6]

	// Validate: byte2 should be 0x0A and byte1-2 should equal byte3
	if byte2 != 0x0A {
		// Skip this delimiter and continue searching
		return "", delimiterIdx + 1, nil
	}

	if int(byte1)-2 != int(byte3) {
		// Skip this delimiter and continue searching
		return "", delimiterIdx + 1, nil
	}

	length := int(byte3)
	chunkStart := delimiterIdx + 7
	chunkEnd := chunkStart + length

	if len(buffer) < chunkEnd {
		return "", 0, nil // Need more data
	}

	text := string(buffer[chunkStart:chunkEnd])
	return text, chunkEnd, nil
}

// GenerateChecksum generates the x-cursor-checksum header value
func GenerateChecksum(token string) string {
	// The checksum format appears to be: hash1/hash2
	// This is a simplified implementation - the actual algorithm may be more complex
	hash1 := sha256.Sum256([]byte(token))
	hash2 := sha256.Sum256([]byte(token + "cursor"))

	return hex.EncodeToString(hash1[:])[:64] + "/" + hex.EncodeToString(hash2[:])[:64]
}

// truncateMessages truncates messages to fit within max input length
func (s *CursorService) truncateMessages(messages []models.Message) []models.Message {
	if len(messages) == 0 || s.config.MaxInputLength <= 0 {
		return messages
	}

	maxLength := s.config.MaxInputLength
	total := 0
	for _, msg := range messages {
		total += len(msg.GetStringContent())
	}

	if total <= maxLength {
		return messages
	}

	var result []models.Message
	startIdx := 0

	if strings.EqualFold(messages[0].Role, "system") {
		result = append(result, messages[0])
		maxLength -= len(messages[0].GetStringContent())
		if maxLength < 0 {
			maxLength = 0
		}
		startIdx = 1
	}

	current := 0
	collected := make([]models.Message, 0, len(messages)-startIdx)
	for i := len(messages) - 1; i >= startIdx; i-- {
		msg := messages[i]
		msgLen := len(msg.GetStringContent())
		if msgLen == 0 {
			continue
		}
		if current+msgLen > maxLength {
			continue
		}
		collected = append(collected, msg)
		current += msgLen
	}

	for i, j := 0, len(collected)-1; i < j; i, j = i+1, j-1 {
		collected[i], collected[j] = collected[j], collected[i]
	}

	return append(result, collected...)
}

// Protobuf message types (embedded for simplicity - in production, use generated code)

type ChatRequest struct {
	Message        []*ChatMessage
	Unknown        []byte
	Paths          string
	Model          *ModelInfo
	TraceId        string
	Unknown1       uint64
	Unknown2       uint64
	ConversationId string
	Unknown4       uint64
	Unknown5       uint64
	Unknown6       uint64
	Unknown7       uint64
	Unknown8       uint64
}

func (x *ChatRequest) ProtoMessage() {}

func (x *ChatRequest) Reset() { *x = ChatRequest{} }

func (x *ChatRequest) String() string { return fmt.Sprintf("%+v", x) }

type ModelInfo struct {
	Model   string
	Unknown []byte
}

func (x *ModelInfo) ProtoMessage() {}

func (x *ModelInfo) Reset() { *x = ModelInfo{} }

func (x *ModelInfo) String() string { return fmt.Sprintf("%+v", x) }

type ChatMessage struct {
	Message string
	Role    uint64
	Uuid    string
}

func (x *ChatMessage) ProtoMessage() {}

func (x *ChatMessage) Reset() { *x = ChatMessage{} }

func (x *ChatMessage) String() string { return fmt.Sprintf("%+v", x) }

// Manual protobuf encoding since we're not using protoc
func (x *ChatRequest) Marshal() ([]byte, error) {
	var buf bytes.Buffer

	// Field 2: messages (repeated)
	for _, msg := range x.Message {
		msgBytes, err := msg.Marshal()
		if err != nil {
			return nil, err
		}
		buf.WriteByte(0x12) // field 2, wire type 2 (length-delimited)
		writeVarint(&buf, uint64(len(msgBytes)))
		buf.Write(msgBytes)
	}

	// Field 4: unknown bytes
	if len(x.Unknown) > 0 {
		buf.WriteByte(0x22) // field 4, wire type 2
		writeVarint(&buf, uint64(len(x.Unknown)))
		buf.Write(x.Unknown)
	}

	// Field 5: paths
	if x.Paths != "" {
		buf.WriteByte(0x2a) // field 5, wire type 2
		writeVarint(&buf, uint64(len(x.Paths)))
		buf.WriteString(x.Paths)
	}

	// Field 7: model
	if x.Model != nil {
		modelBytes, err := x.Model.Marshal()
		if err != nil {
			return nil, err
		}
		buf.WriteByte(0x3a) // field 7, wire type 2
		writeVarint(&buf, uint64(len(modelBytes)))
		buf.Write(modelBytes)
	}

	// Field 9: trace_id
	if x.TraceId != "" {
		buf.WriteByte(0x4a) // field 9, wire type 2
		writeVarint(&buf, uint64(len(x.TraceId)))
		buf.WriteString(x.TraceId)
	}

	// Field 13: unknown1
	if x.Unknown1 != 0 {
		buf.WriteByte(0x68) // field 13, wire type 0
		writeVarint(&buf, x.Unknown1)
	}

	// Field 14: unknown2
	if x.Unknown2 != 0 {
		buf.WriteByte(0x70) // field 14, wire type 0
		writeVarint(&buf, x.Unknown2)
	}

	// Field 15: conversation_id
	if x.ConversationId != "" {
		buf.WriteByte(0x7a) // field 15, wire type 2
		writeVarint(&buf, uint64(len(x.ConversationId)))
		buf.WriteString(x.ConversationId)
	}

	// Field 16: unknown4
	if x.Unknown4 != 0 {
		buf.WriteByte(0x80) // field 16
		buf.WriteByte(0x01)
		writeVarint(&buf, x.Unknown4)
	}

	// Field 22: unknown5
	if x.Unknown5 != 0 {
		buf.WriteByte(0xb0) // field 22
		buf.WriteByte(0x01)
		writeVarint(&buf, x.Unknown5)
	}

	// Field 24: unknown6
	if x.Unknown6 != 0 {
		buf.WriteByte(0xc0) // field 24
		buf.WriteByte(0x01)
		writeVarint(&buf, x.Unknown6)
	}

	// Field 28: unknown7
	if x.Unknown7 != 0 {
		buf.WriteByte(0xe0) // field 28
		buf.WriteByte(0x01)
		writeVarint(&buf, x.Unknown7)
	}

	// Field 29: unknown8
	if x.Unknown8 != 0 {
		buf.WriteByte(0xe8) // field 29
		buf.WriteByte(0x01)
		writeVarint(&buf, x.Unknown8)
	}

	return buf.Bytes(), nil
}

func (x *ModelInfo) Marshal() ([]byte, error) {
	var buf bytes.Buffer

	// Field 1: model
	if x.Model != "" {
		buf.WriteByte(0x0a) // field 1, wire type 2
		writeVarint(&buf, uint64(len(x.Model)))
		buf.WriteString(x.Model)
	}

	// Field 4: unknown bytes
	if len(x.Unknown) > 0 {
		buf.WriteByte(0x22) // field 4, wire type 2
		writeVarint(&buf, uint64(len(x.Unknown)))
		buf.Write(x.Unknown)
	}

	return buf.Bytes(), nil
}

func (x *ChatMessage) Marshal() ([]byte, error) {
	var buf bytes.Buffer

	// Field 1: message
	if x.Message != "" {
		buf.WriteByte(0x0a) // field 1, wire type 2
		writeVarint(&buf, uint64(len(x.Message)))
		buf.WriteString(x.Message)
	}

	// Field 2: role
	if x.Role != 0 {
		buf.WriteByte(0x10) // field 2, wire type 0
		writeVarint(&buf, x.Role)
	}

	// Field 13: uuid
	if x.Uuid != "" {
		buf.WriteByte(0x6a) // field 13, wire type 2
		writeVarint(&buf, uint64(len(x.Uuid)))
		buf.WriteString(x.Uuid)
	}

	return buf.Bytes(), nil
}

func writeVarint(buf *bytes.Buffer, v uint64) {
	for v >= 0x80 {
		buf.WriteByte(byte(v) | 0x80)
		v >>= 7
	}
	buf.WriteByte(byte(v))
}

// Re-implement buildProtobufRequest using manual marshaling
func (s *CursorService) buildProtobufRequestManual(request *models.ChatCompletionRequest) ([]byte, error) {
	// Convert OpenAI messages to Cursor protobuf format
	messages := make([]*ChatMessage, 0, len(request.Messages))
	msgUUID := uuid.New().String()

	for _, msg := range request.Messages {
		role := uint64(1) // user
		if msg.Role == "assistant" || msg.Role == "system" {
			role = 2
		}

		content := msg.GetStringContent()
		if s.config.SystemPromptInject != "" && msg.Role == "system" {
			content = content + "\n" + s.config.SystemPromptInject
		}

		messages = append(messages, &ChatMessage{
			Message: content,
			Role:    role,
			Uuid:    msgUUID,
		})
	}

	// Build request
	conversationID := uuid.New().String()
	traceID := uuid.New().String()

	req := &ChatRequest{
		Message:        messages,
		Unknown:        []byte{},
		Paths:          s.config.CursorWorkingDir,
		Model:          &ModelInfo{Model: request.Model, Unknown: []byte{}},
		TraceId:        traceID,
		Unknown1:       0,
		Unknown2:       0,
		ConversationId: conversationID,
		Unknown4:       1,
		Unknown5:       0,
		Unknown6:       0,
		Unknown7:       0,
		Unknown8:       0,
	}

	return req.Marshal()
}
