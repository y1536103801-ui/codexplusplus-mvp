package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"
)

const gatewayMetadataCaptureBytes = 64 << 10
const gatewayJSONMaxDepth = 256
const gatewayZstdMaxMemoryBytes = 256 << 20

type gatewayRequestMetadata struct {
	Model           string
	RequestID       string
	SessionID       string
	Stream          bool
	MaxOutputTokens int64
}

type gatewayRequestSource struct {
	path     string
	memory   []byte
	size     int64
	metadata gatewayRequestMetadata
}

func newMemoryGatewayRequestSource(body []byte) *gatewayRequestSource {
	return &gatewayRequestSource{memory: body, size: int64(len(body))}
}

func (s *gatewayRequestSource) Open() (io.ReadCloser, error) {
	if s == nil {
		return nil, errors.New("gateway_request_body_unavailable")
	}
	if s.path != "" {
		return os.Open(s.path)
	}
	return io.NopCloser(bytes.NewReader(s.memory)), nil
}

func (s *gatewayRequestSource) Bytes() ([]byte, error) {
	reader, err := s.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}

func (s *gatewayRequestSource) Close() {
	if s != nil && s.path != "" {
		_ = os.Remove(s.path)
		s.path = ""
	}
}

func prepareGatewayRequest(r *http.Request) (*gatewayRequestSource, *gatewayBodyReadError) {
	return prepareGatewayRequestWithinLimits(r, maxGatewayEncodedBodyBytes, maxGatewayDecodedBodyBytes)
}

func prepareGatewayRequestWithinLimits(r *http.Request, encodedLimit, decodedLimit int64) (*gatewayRequestSource, *gatewayBodyReadError) {
	if r == nil || r.Body == nil {
		return nil, classifyGatewayBodyReadError(errors.New("empty request body"))
	}
	if encodedLimit <= 0 || decodedLimit <= 0 {
		return nil, classifyGatewayBodyReadError(errGatewayBodyTooLarge)
	}
	if r.ContentLength > encodedLimit {
		return nil, classifyGatewayBodyReadError(errGatewayBodyTooLarge)
	}

	tempDir := strings.TrimSpace(os.Getenv("CODEXPPP_GATEWAY_TEMP_DIR"))
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		return nil, gatewayStorageError(err)
	}
	file, err := os.CreateTemp(tempDir, "codexppp-gateway-*.json")
	if err != nil {
		return nil, gatewayStorageError(err)
	}
	path := file.Name()
	remove := true
	defer func() {
		_ = file.Close()
		if remove {
			_ = os.Remove(path)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return nil, gatewayStorageError(err)
	}

	encoded := &io.LimitedReader{R: r.Body, N: encodedLimit + 1}
	var decoded io.Reader = encoded
	var decoder *zstd.Decoder
	encoding := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding")))
	switch encoding {
	case "", "identity":
	case "zstd":
		decoder, err = zstd.NewReader(
			encoded,
			zstd.WithDecoderConcurrency(1),
			zstd.WithDecoderMaxWindow(uint64(decodedLimit)),
			zstd.WithDecoderMaxMemory(gatewayZstdMaxMemoryBytes),
		)
		if err != nil {
			return nil, classifyGatewayBodyReadError(err)
		}
		defer decoder.Close()
		decoded = decoder
	default:
		return nil, &gatewayBodyReadError{
			Status: http.StatusUnsupportedMediaType,
			Code:   "unsupported_content_encoding",
			Cause:  fmt.Errorf("unsupported content encoding %q", encoding),
		}
	}

	decodedLimited := &io.LimitedReader{R: decoded, N: decodedLimit + 1}
	writer := bufio.NewWriterSize(file, 64<<10)
	metadata, parseErr := sanitizeGatewayJSONStream(decodedLimited, writer)
	if parseErr == nil {
		parseErr = writer.Flush()
	}
	if decodedLimited.N == 0 || encoded.N == 0 {
		return nil, classifyGatewayBodyReadError(errGatewayBodyTooLarge)
	}
	if parseErr != nil {
		if errors.Is(parseErr, errGatewayBodyTooLarge) {
			return nil, classifyGatewayBodyReadError(parseErr)
		}
		if errors.Is(parseErr, os.ErrPermission) {
			return nil, gatewayStorageError(parseErr)
		}
		return nil, classifyGatewayBodyReadError(parseErr)
	}
	if err := file.Close(); err != nil {
		return nil, gatewayStorageError(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, gatewayStorageError(err)
	}
	remove = false
	return &gatewayRequestSource{path: path, size: info.Size(), metadata: metadata}, nil
}

func gatewayStorageError(err error) *gatewayBodyReadError {
	return &gatewayBodyReadError{Status: http.StatusInsufficientStorage, Code: "request_storage_unavailable", Cause: err}
}

var gatewayRemovedRequestFields = map[string]struct{}{
	"usage": {}, "inputTokens": {}, "input_tokens": {}, "cachedInputTokens": {}, "cached_input_tokens": {},
	"outputTokens": {}, "output_tokens": {}, "totalTokens": {}, "total_tokens": {},
	"requestId": {}, "request_id": {}, "idempotencyKey": {}, "idempotency_key": {},
	"route": {}, "routeId": {}, "route_id": {}, "apiKey": {}, "api_key": {}, "apiKeyId": {}, "api_key_id": {},
	"upstream": {}, "upstreamId": {}, "upstream_id": {}, "gatewayPath": {}, "gateway_path": {},
	"proxy": {}, "endpoint": {}, "base_url": {}, "baseUrl": {},
}

var gatewayCapturedRequestFields = map[string]struct{}{
	"model": {}, "requestId": {}, "request_id": {}, "idempotencyKey": {}, "idempotency_key": {},
	"stream": {}, "max_output_tokens": {}, "max_completion_tokens": {},
	"session_id": {}, "sessionId": {}, "thread_id": {}, "threadId": {}, "conversation_id": {}, "conversationId": {},
	"conversation": {}, "codex_session_id": {}, "codexSessionId": {}, "metadata": {},
}

func sanitizeGatewayJSONStream(src io.Reader, dst io.Writer) (gatewayRequestMetadata, error) {
	parser := &gatewayJSONParser{reader: bufio.NewReaderSize(src, 64<<10)}
	first, err := parser.nextNonSpace()
	if err != nil || first != '{' {
		return gatewayRequestMetadata{}, errors.New("invalid_json")
	}
	if _, err := dst.Write([]byte{'{'}); err != nil {
		return gatewayRequestMetadata{}, err
	}

	metadata := gatewayRequestMetadata{}
	written := 0
	next, err := parser.nextNonSpace()
	if err != nil {
		return metadata, errors.New("invalid_json")
	}
	if next == '}' {
		if _, err := dst.Write([]byte{'}'}); err != nil {
			return metadata, err
		}
		if err := parser.requireEOF(); err != nil {
			return metadata, err
		}
		return metadata, nil
	}
	if err := parser.reader.UnreadByte(); err != nil {
		return metadata, err
	}

	for {
		var keyRaw limitedGatewayBuffer
		keyRaw.limit = gatewayMetadataCaptureBytes
		if err := parser.copyValue(&keyRaw, 1); err != nil || keyRaw.overflow || len(keyRaw.data) == 0 || keyRaw.data[0] != '"' {
			return metadata, errors.New("invalid_json")
		}
		var key string
		if err := json.Unmarshal(keyRaw.data, &key); err != nil {
			return metadata, errors.New("invalid_json")
		}
		colon, err := parser.nextNonSpace()
		if err != nil || colon != ':' {
			return metadata, errors.New("invalid_json")
		}

		_, removed := gatewayRemovedRequestFields[key]
		_, captured := gatewayCapturedRequestFields[key]
		var capture limitedGatewayBuffer
		capture.limit = gatewayMetadataCaptureBytes
		var valueWriter io.Writer = io.Discard
		if !removed {
			if written > 0 {
				if _, err := dst.Write([]byte{','}); err != nil {
					return metadata, err
				}
			}
			if _, err := dst.Write(keyRaw.data); err != nil {
				return metadata, err
			}
			if _, err := dst.Write([]byte{':'}); err != nil {
				return metadata, err
			}
			written++
			valueWriter = dst
		}
		if captured {
			valueWriter = io.MultiWriter(valueWriter, &capture)
		}
		if err := parser.copyValue(valueWriter, 1); err != nil {
			return metadata, err
		}
		if captured && !capture.overflow {
			captureGatewayRequestMetadata(&metadata, key, capture.data)
		}

		separator, err := parser.nextNonSpace()
		if err != nil {
			return metadata, errors.New("invalid_json")
		}
		switch separator {
		case ',':
			continue
		case '}':
			if _, err := dst.Write([]byte{'}'}); err != nil {
				return metadata, err
			}
			if err := parser.requireEOF(); err != nil {
				return metadata, err
			}
			return metadata, nil
		default:
			return metadata, errors.New("invalid_json")
		}
	}
}

func captureGatewayRequestMetadata(metadata *gatewayRequestMetadata, key string, raw []byte) {
	if metadata == nil || len(raw) == 0 {
		return
	}
	switch key {
	case "model":
		_ = json.Unmarshal(raw, &metadata.Model)
	case "requestId", "request_id", "idempotencyKey", "idempotency_key":
		if metadata.RequestID == "" {
			_ = json.Unmarshal(raw, &metadata.RequestID)
		}
	case "stream":
		_ = json.Unmarshal(raw, &metadata.Stream)
	case "max_output_tokens", "max_completion_tokens":
		if metadata.MaxOutputTokens == 0 {
			var number json.Number
			if err := json.Unmarshal(raw, &number); err == nil {
				metadata.MaxOutputTokens, _ = strconv.ParseInt(number.String(), 10, 64)
			}
		}
	case "session_id", "sessionId", "thread_id", "threadId", "conversation_id", "conversationId", "codex_session_id", "codexSessionId":
		if metadata.SessionID == "" {
			var value string
			if json.Unmarshal(raw, &value) == nil {
				metadata.SessionID = gatewaySessionIDFromValue(value)
			}
		}
	case "conversation":
		if metadata.SessionID == "" {
			var value any
			if json.Unmarshal(raw, &value) == nil {
				switch typed := value.(type) {
				case string:
					metadata.SessionID = gatewaySessionIDFromValue(typed)
				case map[string]any:
					metadata.SessionID = gatewaySessionIDFromValue(stringField(typed, "id"))
				}
			}
		}
	case "metadata":
		if metadata.SessionID == "" {
			var value map[string]any
			if json.Unmarshal(raw, &value) == nil {
				for _, field := range []string{"session_id", "sessionId", "thread_id", "threadId", "conversation_id", "conversationId"} {
					if candidate := gatewaySessionIDFromValue(stringField(value, field)); candidate != "" {
						metadata.SessionID = candidate
						break
					}
				}
			}
		}
	}
}

type limitedGatewayBuffer struct {
	data     []byte
	limit    int
	overflow bool
}

func (w *limitedGatewayBuffer) Write(p []byte) (int, error) {
	if !w.overflow {
		remaining := w.limit - len(w.data)
		if remaining < len(p) {
			if remaining > 0 {
				w.data = append(w.data, p[:remaining]...)
			}
			w.overflow = true
		} else {
			w.data = append(w.data, p...)
		}
	}
	return len(p), nil
}

type gatewayJSONParser struct {
	reader *bufio.Reader
}

func (p *gatewayJSONParser) nextNonSpace() (byte, error) {
	for {
		b, err := p.reader.ReadByte()
		if err != nil {
			return 0, err
		}
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return b, nil
		}
	}
}

func (p *gatewayJSONParser) requireEOF() error {
	for {
		b, err := p.reader.ReadByte()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		switch b {
		case ' ', '\t', '\r', '\n':
		default:
			return errors.New("invalid_json")
		}
	}
}

func (p *gatewayJSONParser) copyValue(dst io.Writer, depth int) error {
	if depth > gatewayJSONMaxDepth {
		return errors.New("invalid_json")
	}
	first, err := p.nextNonSpace()
	if err != nil {
		return errors.New("invalid_json")
	}
	switch first {
	case '"':
		return p.copyString(dst)
	case '{':
		return p.copyObject(dst, depth+1)
	case '[':
		return p.copyArray(dst, depth+1)
	case 't':
		return p.copyLiteral(dst, "true")
	case 'f':
		return p.copyLiteral(dst, "false")
	case 'n':
		return p.copyLiteral(dst, "null")
	default:
		if first == '-' || (first >= '0' && first <= '9') {
			return p.copyNumber(dst, first)
		}
		return errors.New("invalid_json")
	}
}

func (p *gatewayJSONParser) copyString(dst io.Writer) error {
	if _, err := dst.Write([]byte{'"'}); err != nil {
		return err
	}
	escape := false
	unicodeDigits := 0
	for {
		chunk, readErr := p.reader.ReadSlice('"')
		if len(chunk) > 0 {
			terminated := false
			for _, b := range chunk {
				if unicodeDigits > 0 {
					if !isHexByte(b) {
						return errors.New("invalid_json")
					}
					unicodeDigits--
					continue
				}
				if escape {
					escape = false
					switch b {
					case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
					case 'u':
						unicodeDigits = 4
					default:
						return errors.New("invalid_json")
					}
					continue
				}
				switch {
				case b == '\\':
					escape = true
				case b == '"':
					terminated = true
				case b < 0x20:
					return errors.New("invalid_json")
				}
			}
			if _, err := dst.Write(chunk); err != nil {
				return err
			}
			if terminated {
				return nil
			}
		}
		if readErr != nil && !errors.Is(readErr, bufio.ErrBufferFull) {
			return errors.New("invalid_json")
		}
	}
}

func (p *gatewayJSONParser) copyObject(dst io.Writer, depth int) error {
	if _, err := dst.Write([]byte{'{'}); err != nil {
		return err
	}
	next, err := p.nextNonSpace()
	if err != nil {
		return errors.New("invalid_json")
	}
	if next == '}' {
		_, err = dst.Write([]byte{'}'})
		return err
	}
	if err := p.reader.UnreadByte(); err != nil {
		return err
	}
	written := 0
	for {
		keyStart, err := p.nextNonSpace()
		if err != nil || keyStart != '"' {
			return errors.New("invalid_json")
		}
		if written > 0 {
			if _, err := dst.Write([]byte{','}); err != nil {
				return err
			}
		}
		if err := p.copyString(dst); err != nil {
			return err
		}
		colon, err := p.nextNonSpace()
		if err != nil || colon != ':' {
			return errors.New("invalid_json")
		}
		if _, err := dst.Write([]byte{':'}); err != nil {
			return err
		}
		if err := p.copyValue(dst, depth); err != nil {
			return err
		}
		written++
		separator, err := p.nextNonSpace()
		if err != nil {
			return errors.New("invalid_json")
		}
		if separator == '}' {
			_, err = dst.Write([]byte{'}'})
			return err
		}
		if separator != ',' {
			return errors.New("invalid_json")
		}
	}
}

func (p *gatewayJSONParser) copyArray(dst io.Writer, depth int) error {
	if _, err := dst.Write([]byte{'['}); err != nil {
		return err
	}
	next, err := p.nextNonSpace()
	if err != nil {
		return errors.New("invalid_json")
	}
	if next == ']' {
		_, err = dst.Write([]byte{']'})
		return err
	}
	if err := p.reader.UnreadByte(); err != nil {
		return err
	}
	written := 0
	for {
		if written > 0 {
			if _, err := dst.Write([]byte{','}); err != nil {
				return err
			}
		}
		if err := p.copyValue(dst, depth); err != nil {
			return err
		}
		written++
		separator, err := p.nextNonSpace()
		if err != nil {
			return errors.New("invalid_json")
		}
		if separator == ']' {
			_, err = dst.Write([]byte{']'})
			return err
		}
		if separator != ',' {
			return errors.New("invalid_json")
		}
	}
}

func (p *gatewayJSONParser) copyLiteral(dst io.Writer, literal string) error {
	remaining := literal[1:]
	buf := make([]byte, len(remaining))
	if _, err := io.ReadFull(p.reader, buf); err != nil || string(buf) != remaining {
		return errors.New("invalid_json")
	}
	_, err := io.WriteString(dst, literal)
	return err
}

func (p *gatewayJSONParser) copyNumber(dst io.Writer, first byte) error {
	var raw []byte
	raw = append(raw, first)
	for {
		b, err := p.reader.ReadByte()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if strings.ContainsRune("0123456789+-.eE", rune(b)) {
			raw = append(raw, b)
			if len(raw) > 256 {
				return errors.New("invalid_json")
			}
			continue
		}
		if err := p.reader.UnreadByte(); err != nil {
			return err
		}
		break
	}
	if !validJSONNumber(string(raw)) {
		return errors.New("invalid_json")
	}
	_, err := dst.Write(raw)
	return err
}

func validJSONNumber(raw string) bool {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var number json.Number
	if err := dec.Decode(&number); err != nil {
		return false
	}
	var extra any
	return dec.Decode(&extra) == io.EOF
}

func isHexByte(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func cleanupGatewayTempFiles() {
	tempDir := strings.TrimSpace(os.Getenv("CODEXPPP_GATEWAY_TEMP_DIR"))
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	matches, _ := filepath.Glob(filepath.Join(tempDir, "codexppp-gateway-*.json"))
	for _, path := range matches {
		_ = os.Remove(path)
	}
}
