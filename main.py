"""Cursor2API - Convert Cursor IDE API to OpenAI-compatible API."""
import uvicorn
from fastapi import FastAPI, Request
from fastapi.staticfiles import StaticFiles
from fastapi.responses import FileResponse, HTMLResponse
from fastapi.middleware.cors import CORSMiddleware

from app.config import settings
from app.routes import router

# Create FastAPI application
app = FastAPI(
    title="Cursor2API",
    description="将 Cursor IDE API 转换为 OpenAI 兼容 API 的代理服务",
    version="2.0.0",
    docs_url="/docs" if settings.debug else None,
    redoc_url="/redoc" if settings.debug else None,
)

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include API routes
app.include_router(router)

# Mount static files
app.mount("/static", StaticFiles(directory="static"), name="static")


@app.get("/", response_class=HTMLResponse)
async def root():
    """Serve the main UI page."""
    return FileResponse("static/index.html")


@app.get("/favicon.ico")
async def favicon():
    """Serve favicon."""
    return FileResponse("static/favicon.ico", media_type="image/x-icon")


if __name__ == "__main__":
    print(f"""
╔═══════════════════════════════════════════════════════════╗
║                    Cursor2API v2.0.0                      ║
║           Cursor IDE API → OpenAI Compatible API          ║
╠═══════════════════════════════════════════════════════════╣
║  服务地址: http://localhost:{settings.port}                       ║
║  API密钥: {settings.api_key[:20]}{'...' if len(settings.api_key) > 20 else ''}                              
║  Cursor Token: {'已配置 ✓' if settings.get_clean_token() else '未配置 ✗'}                               ║
║  支持模型: {len(settings.get_models())} 个                                     ║
╚═══════════════════════════════════════════════════════════╝
    """)
    
    uvicorn.run(
        "main:app",
        host="0.0.0.0",
        port=settings.port,
        reload=settings.debug
    )

