from aiogram import Router
from aiogram.types import ErrorEvent
from bot.logger import logger

router = Router()

@router.error()
async def error_handler(event: ErrorEvent):
    """Глобальный обработчик ошибок"""
    
    logger.error(
        f"Error occurred: {event.exception.__class__.__name__}: {event.exception}",
        exc_info=True
    )
    
    # Можно отправить сообщение админу или в мониторинг
    # Но пользователю просто логируем ошибку
    
    return True  # Предотвращает дальнейшую обработку ошибки