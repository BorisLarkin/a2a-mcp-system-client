"""
Базовые классы обработчиков для переиспользования логики
"""

from abc import ABC, abstractmethod
from typing import Optional, Dict, Any
from aiogram import BaseMiddleware
from aiogram.types import Message, CallbackQuery
from bot.logger import logger
from bot.services.rate_limiter import RateLimiter

class BaseHandler(ABC):
    """Абстрактный базовый класс для всех обработчиков"""
    
    def __init__(self):
        self.rate_limiter = RateLimiter()
    
    @abstractmethod
    async def handle(self, *args, **kwargs):
        """Основной метод обработки"""
        pass
    
    async def _check_rate_limit(self, user_id: int) -> bool:
        """Проверка rate limit для пользователя"""
        if not self.rate_limiter.is_allowed(user_id):
            logger.warning(f"Rate limit exceeded for user {user_id}")
            return False
        return True
    
    async def _log_activity(self, user_id: int, action: str, details: Optional[Dict] = None):
        """Логирование активности пользователя"""
        log_data = {
            "user_id": user_id,
            "action": action,
            "timestamp": self._get_timestamp()
        }
        
        if details:
            log_data.update(details)
        
        logger.info(f"User activity: {log_data}")
    
    @staticmethod
    def _get_timestamp() -> str:
        """Получение временной метки"""
        from datetime import datetime
        return datetime.now().isoformat()
    
    @staticmethod
    def _extract_user_info(update: Any) -> Dict[str, Any]:
        """Извлечение информации о пользователе из обновления"""
        user_info = {}
        
        if isinstance(update, Message):
            user = update.from_user
            user_info = {
                "user_id": user.id,
                "username": user.username,
                "full_name": user.full_name,
                "is_bot": user.is_bot,
                "language_code": user.language_code,
                "chat_id": update.chat.id,
                "chat_type": update.chat.type
            }
        elif isinstance(update, CallbackQuery):
            user = update.from_user
            user_info = {
                "user_id": user.id,
                "username": user.username,
                "full_name": user.full_name,
                "callback_data": update.data
            }
        
        return user_info


class UserActivityMiddleware(BaseMiddleware):
    """Middleware для логирования активности пользователей"""
    
    async def __call__(self, handler, event, data):
        # Логируем входящее событие
        if isinstance(event, Message):
            logger.debug(
                f"Message from {event.from_user.id}: "
                f"{event.text[:50]}{'...' if len(event.text) > 50 else ''}"
            )
        elif isinstance(event, CallbackQuery):
            logger.debug(
                f"Callback from {event.from_user.id}: {event.data}"
            )
        
        # Продолжаем обработку
        return await handler(event, data)


class MaintenanceModeMiddleware(BaseMiddleware):
    """Middleware для режима обслуживания"""
    
    def __init__(self, maintenance_mode: bool = False):
        self.maintenance_mode = maintenance_mode
    
    async def __call__(self, handler, event, data):
        if self.maintenance_mode and isinstance(event, Message):
            # Если режим обслуживания включен
            from bot.config import config
            await event.answer(
                "🔧 Система на техническом обслуживании. "
                "Пожалуйста, повторите попытку позже."
            )
            return
        
        return await handler(event, data)


class MessageLogger:
    """Класс для логирования сообщений"""
    
    @staticmethod
    async def log_incoming(message: Message):
        """Логирование входящего сообщения"""
        log_entry = {
            "type": "incoming",
            "message_id": message.message_id,
            "user_id": message.from_user.id,
            "chat_id": message.chat.id,
            "text": message.text,
            "timestamp": message.date.isoformat() if message.date else None
        }
        
        logger.info(f"Incoming message: {log_entry}")
        
        # Сохраняем в файл для анализа
        await MessageLogger._save_to_file(log_entry)
    
    @staticmethod
    async def log_outgoing(chat_id: int, text: str, success: bool):
        """Логирование исходящего сообщения"""
        log_entry = {
            "type": "outgoing",
            "chat_id": chat_id,
            "text_length": len(text),
            "success": success,
            "timestamp": BaseHandler._get_timestamp()
        }
        
        if not success:
            log_entry["error"] = "Failed to send"
        
        logger.info(f"Outgoing message: {log_entry}")
        await MessageLogger._save_to_file(log_entry)
    
    @staticmethod
    async def _save_to_file(log_entry: Dict):
        """Сохранение лога в файл (опционально)"""
        import json
        from pathlib import Path
        
        log_dir = Path("logs/messages")
        log_dir.mkdir(parents=True, exist_ok=True)
        
        log_file = log_dir / "messages.jsonl"
        
        try:
            with open(log_file, "a", encoding="utf-8") as f:
                f.write(json.dumps(log_entry, ensure_ascii=False) + "\n")
        except Exception as e:
            logger.error(f"Failed to save log to file: {e}")