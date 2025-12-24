"""API routes for OpenAI-compatible endpoints."""
import time
import uuid
import json
from typing import Optional
from fastapi import APIRouter, HTTPException, Header, Request
from fastapi.responses import StreamingResponse, JSONResponse
from sse_starlette.sse import EventSourceResponse

from .config import settings
from .models import (
    ChatCompletionRequest,
    ChatCompletionResponse,
    ChatCompletionStreamResponse,
    ModelListResponse,
    ModelInfo,
    Choice,
    Message,
    Usage,
    ErrorResponse,
    ErrorDetail,
)
from .cursor_client import cursor_client

router = APIRouter()


def verify_api_key(authorization: Optional[str]) -> bool:
    """Verify API key from Authorization header."""
    if not authorization:
        return False
    
    # Handle both "Bearer xxx" and "xxx" formats
    token = authorization
    if authorization.startswith("Bearer "):
        token = authorization[7:]
    
    return token == settings.api_key


@router.get("/v1/models")
async def list_models(authorization: Optional[str] = Header(None)):
    """List available models."""
    if not verify_api_key(authorization):
        raise HTTPException(status_code=401, detail="Invalid API key")
    
    models = []
    for model_id in settings.get_models():
        # Determine provider
        owned_by = "cursor"
        if "claude" in model_id.lower():
            owned_by = "anthropic"
        elif "gpt" in model_id.lower() or model_id.startswith("o"):
            owned_by = "openai"
        elif "gemini" in model_id.lower():
            owned_by = "google"
        elif "deepseek" in model_id.lower():
            owned_by = "deepseek"
        elif "grok" in model_id.lower():
            owned_by = "xai"
        elif "kimi" in model_id.lower():
            owned_by = "moonshot"
        
        models.append(ModelInfo(id=model_id, owned_by=owned_by))
    
    return ModelListResponse(data=models)


@router.post("/v1/chat/completions")
async def chat_completions(
    request: ChatCompletionRequest,
    authorization: Optional[str] = Header(None)
):
    """Create chat completion."""
    if not verify_api_key(authorization):
        raise HTTPException(status_code=401, detail="Invalid API key")
    
    # Check if Cursor token is configured
    if not settings.get_clean_token():
        raise HTTPException(
            status_code=500,
            detail="CURSOR_TOKEN is not configured. Please set it in .env file."
        )
    
    response_id = f"chatcmpl-{uuid.uuid4().hex[:29]}"
    created = int(time.time())
    
    if request.stream:
        return await stream_chat_completion(request, response_id, created)
    else:
        return await non_stream_chat_completion(request, response_id, created)


async def stream_chat_completion(
    request: ChatCompletionRequest,
    response_id: str,
    created: int
):
    """Handle streaming chat completion."""
    
    async def generate():
        try:
            async for chunk in cursor_client.chat_completion_stream(
                request.messages,
                request.model
            ):
                if chunk:
                    response = ChatCompletionStreamResponse(
                        id=response_id,
                        created=created,
                        model=request.model,
                        choices=[
                            Choice(
                                index=0,
                                delta={"content": chunk},
                                finish_reason=None
                            )
                        ]
                    )
                    yield {"data": response.model_dump_json()}
            
            # Send final chunk with finish_reason
            final_response = ChatCompletionStreamResponse(
                id=response_id,
                created=created,
                model=request.model,
                choices=[
                    Choice(
                        index=0,
                        delta={},
                        finish_reason="stop"
                    )
                ]
            )
            yield {"data": final_response.model_dump_json()}
            yield {"data": "[DONE]"}
            
        except Exception as e:
            error_data = {
                "error": {
                    "message": str(e),
                    "type": "api_error",
                    "code": "cursor_api_error"
                }
            }
            yield {"data": json.dumps(error_data)}
            yield {"data": "[DONE]"}
    
    return EventSourceResponse(generate())


async def non_stream_chat_completion(
    request: ChatCompletionRequest,
    response_id: str,
    created: int
):
    """Handle non-streaming chat completion."""
    try:
        full_response = await cursor_client.chat_completion(
            request.messages,
            request.model
        )
        
        return ChatCompletionResponse(
            id=response_id,
            created=created,
            model=request.model,
            choices=[
                Choice(
                    index=0,
                    message=Message(role="assistant", content=full_response),
                    finish_reason="stop"
                )
            ],
            usage=Usage(
                prompt_tokens=0,
                completion_tokens=0,
                total_tokens=0
            )
        )
    
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/health")
async def health_check():
    """Health check endpoint."""
    return {
        "status": "healthy",
        "version": "2.0.0",
        "cursor_configured": bool(settings.get_clean_token())
    }


@router.get("/status")
async def status():
    """Get service status."""
    return {
        "status": "running",
        "models_count": len(settings.get_models()),
        "cursor_token_set": bool(settings.get_clean_token()),
        "cursor_api_url": settings.cursor_api_url,
        "cursor_version": settings.cursor_version
    }

