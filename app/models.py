"""Data models for OpenAI-compatible API."""
from typing import List, Optional, Union, Dict, Any
from pydantic import BaseModel, Field
import time
import uuid


class Message(BaseModel):
    """Chat message."""
    role: str
    content: Union[str, List[Dict[str, Any]]]
    name: Optional[str] = None
    
    def get_text_content(self) -> str:
        """Extract text content from message."""
        if isinstance(self.content, str):
            return self.content
        elif isinstance(self.content, list):
            texts = []
            for item in self.content:
                if isinstance(item, dict) and item.get("type") == "text":
                    texts.append(item.get("text", ""))
            return "\n".join(texts)
        return ""


class ChatCompletionRequest(BaseModel):
    """OpenAI chat completion request."""
    model: str
    messages: List[Message]
    temperature: Optional[float] = 0.7
    top_p: Optional[float] = 1.0
    n: Optional[int] = 1
    stream: Optional[bool] = False
    max_tokens: Optional[int] = None
    presence_penalty: Optional[float] = 0
    frequency_penalty: Optional[float] = 0
    user: Optional[str] = None


class Choice(BaseModel):
    """Chat completion choice."""
    index: int = 0
    message: Optional[Message] = None
    delta: Optional[Dict[str, str]] = None
    finish_reason: Optional[str] = None


class Usage(BaseModel):
    """Token usage statistics."""
    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: int = 0


class ChatCompletionResponse(BaseModel):
    """OpenAI chat completion response."""
    id: str = Field(default_factory=lambda: f"chatcmpl-{uuid.uuid4().hex[:29]}")
    object: str = "chat.completion"
    created: int = Field(default_factory=lambda: int(time.time()))
    model: str
    choices: List[Choice]
    usage: Optional[Usage] = None


class ChatCompletionStreamResponse(BaseModel):
    """OpenAI chat completion stream response chunk."""
    id: str
    object: str = "chat.completion.chunk"
    created: int
    model: str
    choices: List[Choice]


class ModelInfo(BaseModel):
    """Model information."""
    id: str
    object: str = "model"
    created: int = 1700000000
    owned_by: str = "cursor"


class ModelListResponse(BaseModel):
    """Model list response."""
    object: str = "list"
    data: List[ModelInfo]


class ErrorDetail(BaseModel):
    """Error detail."""
    message: str
    type: str
    param: Optional[str] = None
    code: Optional[str] = None


class ErrorResponse(BaseModel):
    """Error response."""
    error: ErrorDetail

