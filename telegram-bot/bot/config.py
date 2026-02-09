import os
from typing import Optional
from pydantic_settings import BaseSettings
from pydantic import validator
from dotenv import load_dotenv

load_dotenv()

class BotConfig(BaseSettings):
    """Конфигурация бота"""
    
    # Telegram
    bot_token: str
    bot_username: Optional[str] = None
    
    # Local Proxy API
    api_url: str = "http://localhost:8080"
    api_endpoint: str = "/api/v1/tickets"
    api_timeout: int = 10
    
    # Настройки бота
    rate_limit_per_user: int = 5  # сообщений в минуту
    max_message_length: int = 4000
    log_level: str = "INFO"
    
    # Офлайн режим
    offline_response: str = "Спасибо за обращение! Мы получили ваше сообщение и ответим при первой возможности."
    maintenance_mode: bool = False
    
    # Redis (опционально)
    redis_url: Optional[str] = None
    redis_enabled: bool = False
    
    @validator('bot_token')
    def validate_bot_token(cls, v):
        if not v or ':' not in v:
            raise ValueError('Invalid bot token format')
        return v
    
    @validator('api_url')
    def validate_api_url(cls, v):
        if not v.startswith(('http://', 'https://')):
            raise ValueError('API URL must start with http:// or https://')
        return v.rstrip('/')
    
    class Config:
        env_file = '.env'
        env_prefix = ''

config = BotConfig()