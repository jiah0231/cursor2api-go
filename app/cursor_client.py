"""Cursor IDE gRPC-Web client implementation."""
import struct
import uuid
import hashlib
import asyncio
from typing import AsyncGenerator, List, Optional
import httpx
from .config import settings
from .models import Message


class ProtobufEncoder:
    """Manual protobuf encoder for Cursor API requests."""
    
    @staticmethod
    def encode_varint(value: int) -> bytes:
        """Encode an integer as a varint."""
        result = []
        while value > 0x7f:
            result.append((value & 0x7f) | 0x80)
            value >>= 7
        result.append(value)
        return bytes(result) if result else b'\x00'
    
    @staticmethod
    def encode_string(field_num: int, value: str) -> bytes:
        """Encode a string field."""
        if not value:
            return b''
        encoded = value.encode('utf-8')
        tag = (field_num << 3) | 2  # wire type 2 = length-delimited
        return ProtobufEncoder.encode_varint(tag) + ProtobufEncoder.encode_varint(len(encoded)) + encoded
    
    @staticmethod
    def encode_bytes(field_num: int, value: bytes) -> bytes:
        """Encode a bytes field."""
        if not value:
            return b''
        tag = (field_num << 3) | 2
        return ProtobufEncoder.encode_varint(tag) + ProtobufEncoder.encode_varint(len(value)) + value
    
    @staticmethod
    def encode_uint64(field_num: int, value: int) -> bytes:
        """Encode a uint64 field."""
        if value == 0:
            return b''
        tag = (field_num << 3) | 0  # wire type 0 = varint
        return ProtobufEncoder.encode_varint(tag) + ProtobufEncoder.encode_varint(value)
    
    @staticmethod
    def encode_message(field_num: int, value: bytes) -> bytes:
        """Encode a nested message field."""
        if not value:
            return b''
        tag = (field_num << 3) | 2
        return ProtobufEncoder.encode_varint(tag) + ProtobufEncoder.encode_varint(len(value)) + value


class CursorMessage:
    """Represents a message in Cursor format."""
    
    def __init__(self, content: str, role: int, msg_uuid: str):
        self.content = content
        self.role = role  # 1 = user, 2 = assistant/system
        self.uuid = msg_uuid
    
    def encode(self) -> bytes:
        """Encode message to protobuf bytes."""
        result = b''
        result += ProtobufEncoder.encode_string(1, self.content)  # message
        result += ProtobufEncoder.encode_uint64(2, self.role)     # role
        result += ProtobufEncoder.encode_string(13, self.uuid)    # uuid
        return result


class CursorModel:
    """Represents model info in Cursor format."""
    
    def __init__(self, model_name: str):
        self.model_name = model_name
    
    def encode(self) -> bytes:
        """Encode model to protobuf bytes."""
        return ProtobufEncoder.encode_string(1, self.model_name)


class CursorRequest:
    """Represents a chat request in Cursor format."""
    
    def __init__(
        self,
        messages: List[CursorMessage],
        model: CursorModel,
        paths: str,
        trace_id: str,
        conversation_id: str
    ):
        self.messages = messages
        self.model = model
        self.paths = paths
        self.trace_id = trace_id
        self.conversation_id = conversation_id
    
    def encode(self) -> bytes:
        """Encode request to protobuf bytes."""
        result = b''
        
        # Field 2: messages (repeated)
        for msg in self.messages:
            msg_bytes = msg.encode()
            result += ProtobufEncoder.encode_message(2, msg_bytes)
        
        # Field 5: paths
        result += ProtobufEncoder.encode_string(5, self.paths)
        
        # Field 7: model
        model_bytes = self.model.encode()
        result += ProtobufEncoder.encode_message(7, model_bytes)
        
        # Field 9: trace_id
        result += ProtobufEncoder.encode_string(9, self.trace_id)
        
        # Field 15: conversation_id
        result += ProtobufEncoder.encode_string(15, self.conversation_id)
        
        # Field 16: unknown4 = 1
        result += ProtobufEncoder.encode_uint64(16, 1)
        
        return result


class CursorClient:
    """Async client for Cursor IDE API."""
    
    def __init__(self):
        self.api_url = settings.cursor_api_url
        self.token = settings.get_clean_token()
        self.timeout = settings.timeout
    
    def _build_headers(self, trace_id: str) -> dict:
        """Build request headers."""
        headers = {
            "User-Agent": "connect-es/1.6.1",
            "Authorization": f"Bearer {self.token}",
            "connect-accept-encoding": "gzip,br",
            "connect-protocol-version": "1",
            "Content-Type": "application/connect+proto",
            "x-amzn-trace-id": f"Root={trace_id}",
            "x-cursor-client-version": settings.cursor_version,
            "x-cursor-timezone": settings.cursor_timezone,
            "x-ghost-mode": str(settings.cursor_ghost_mode).lower(),
            "x-request-id": trace_id,
        }
        
        if settings.cursor_client_key:
            headers["x-client-key"] = settings.cursor_client_key
        
        if settings.cursor_checksum:
            headers["x-cursor-checksum"] = settings.cursor_checksum
        else:
            # Generate a default checksum
            headers["x-cursor-checksum"] = self._generate_checksum()
        
        return headers
    
    def _generate_checksum(self) -> str:
        """Generate x-cursor-checksum header value."""
        # This is a simplified implementation
        hash1 = hashlib.sha256(self.token.encode()).hexdigest()
        hash2 = hashlib.sha256(f"{self.token}cursor".encode()).hexdigest()
        return f"{hash1[:64]}/{hash2[:64]}"
    
    def _build_grpc_envelope(self, data: bytes) -> bytes:
        """Build gRPC-Web envelope with 5-byte length prefix."""
        # Format: 1-byte compression flag + 4-byte big-endian length + data
        return struct.pack(">BI", 0, len(data)) + data
    
    def _convert_messages(self, messages: List[Message]) -> List[CursorMessage]:
        """Convert OpenAI messages to Cursor format."""
        result = []
        msg_uuid = str(uuid.uuid4())
        
        for msg in messages:
            role = 1  # user
            if msg.role in ("assistant", "system"):
                role = 2
            
            content = msg.get_text_content()
            
            # Inject system prompt if configured
            if msg.role == "system" and settings.system_prompt_inject:
                content = f"{content}\n{settings.system_prompt_inject}"
            
            result.append(CursorMessage(content, role, msg_uuid))
        
        return result
    
    async def chat_completion_stream(
        self,
        messages: List[Message],
        model: str
    ) -> AsyncGenerator[str, None]:
        """Stream chat completion from Cursor API."""
        if not self.token:
            raise ValueError("CURSOR_TOKEN is not configured")
        
        # Build request
        trace_id = str(uuid.uuid4())
        conversation_id = str(uuid.uuid4())
        
        cursor_messages = self._convert_messages(messages)
        cursor_model = CursorModel(model)
        
        request = CursorRequest(
            messages=cursor_messages,
            model=cursor_model,
            paths=settings.cursor_working_dir,
            trace_id=trace_id,
            conversation_id=conversation_id
        )
        
        # Encode and wrap in gRPC envelope
        proto_data = request.encode()
        envelope = self._build_grpc_envelope(proto_data)
        
        # Make request
        url = f"{self.api_url}/aiserver.v1.AiService/StreamChat"
        headers = self._build_headers(trace_id)
        
        async with httpx.AsyncClient(timeout=self.timeout) as client:
            async with client.stream("POST", url, content=envelope, headers=headers) as response:
                if response.status_code != 200:
                    error_body = await response.aread()
                    raise Exception(f"Cursor API error: {response.status_code} - {error_body.decode()}")
                
                buffer = b""
                async for chunk in response.aiter_bytes():
                    buffer += chunk
                    
                    # Parse gRPC-Web chunks
                    while True:
                        text, consumed = self._parse_grpc_chunk(buffer)
                        if consumed == 0:
                            break
                        
                        buffer = buffer[consumed:]
                        
                        if text:
                            yield text
    
    def _parse_grpc_chunk(self, buffer: bytes) -> tuple:
        """Parse a gRPC-Web chunk and extract text content."""
        # Look for delimiter pattern: 00 00 00 00
        delimiter = b'\x00\x00\x00\x00'
        idx = buffer.find(delimiter)
        
        if idx == -1 or len(buffer) < idx + 7:
            return "", 0
        
        # Check bytes after delimiter
        byte1 = buffer[idx + 4]
        byte2 = buffer[idx + 5]
        byte3 = buffer[idx + 6]
        
        # Validate: byte2 should be 0x0A
        if byte2 != 0x0A:
            return "", idx + 1
        
        # Validate: byte1 - 2 should equal byte3
        if byte1 - 2 != byte3:
            return "", idx + 1
        
        length = byte3
        chunk_start = idx + 7
        chunk_end = chunk_start + length
        
        if len(buffer) < chunk_end:
            return "", 0
        
        try:
            text = buffer[chunk_start:chunk_end].decode('utf-8')
            return text, chunk_end
        except UnicodeDecodeError:
            return "", chunk_end
    
    async def chat_completion(
        self,
        messages: List[Message],
        model: str
    ) -> str:
        """Get complete chat response from Cursor API."""
        full_response = ""
        async for chunk in self.chat_completion_stream(messages, model):
            full_response += chunk
        return full_response


# Global client instance
cursor_client = CursorClient()

