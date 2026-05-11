"""
Обработчики ошибок.
"""
import logging
from aiogram import Router
from aiogram.types import ErrorEvent

logger = logging.getLogger(__name__)
router = Router()


@router.errors()
async def error_handler(event: ErrorEvent):
    logger.error(f"Update {event.update} caused error: {event.exception}")
    return True  # подавляем ошибку, бот продолжает работать