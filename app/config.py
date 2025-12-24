"""Configuration management for Cursor2API."""
import os
from typing import List
from pydantic_settings import BaseSettings
from pydantic import Field


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""
    
    # Server Configuration
    port: int = Field(default=8002, description="Server port")
    debug: bool = Field(default=False, description="Debug mode")
    
    # API Authentication
    api_key: str = Field(default="sk-cursor2api", description="API key for authentication")
    
    # Supported Models
    models: str = Field(
        default="gpt-4o,claude-3.5-sonnet,claude-3.5-haiku,claude-4-sonnet,gpt-4-turbo,deepseek-r1,gemini-2.5-pro",
        description="Comma-separated list of supported models"
    )
    
    # System Prompt Injection
    system_prompt_inject: str = Field(default="", description="System prompt to inject")
    
    # Request Configuration
    timeout: int = Field(default=120, description="Request timeout in seconds")
    max_input_length: int = Field(default=200000, description="Maximum input length")
    
    # Cursor IDE Client Configuration
    cursor_api_url: str = Field(
        default="https://api2.cursor.sh",
        description="Cursor API base URL"
    )
    cursor_token: str = Field(
        default="",
        description="Cursor session token (WorkosCursorSessionToken)"
    )
    cursor_checksum: str = Field(
        default="",
        description="Cursor checksum header value"
    )
    cursor_client_key: str = Field(default="", description="Cursor client key")
    cursor_version: str = Field(default="0.48.6", description="Cursor client version")
    cursor_timezone: str = Field(default="Asia/Shanghai", description="Timezone")
    cursor_ghost_mode: bool = Field(default=True, description="Ghost mode enabled")
    cursor_working_dir: str = Field(
        default="/c:/Users/Default",
        description="Working directory path"
    )
    
    class Config:
        env_file = ".env"
        env_file_encoding = "utf-8"
        extra = "ignore"
    
    def get_models(self) -> List[str]:
        """Get list of supported models."""
        return [m.strip() for m in self.models.split(",") if m.strip()]
    
    def get_clean_token(self) -> str:
        """Get cleaned Cursor token, handling %3A%3A separator."""
        token = self.cursor_token.strip()
        if not token:
            return ""
        
        # Handle URL-encoded :: separator
        if "%3A%3A" in token:
            parts = token.split("%3A%3A")
            token = parts[-1]
        
        # Handle :: separator
        if "::" in token:
            parts = token.split("::")
            token = parts[-1]
        
        return token.strip()


# Global settings instance
settings = Settings()

