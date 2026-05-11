"""
FastAPI сервер внутри бота для приёма команд от local-proxy:
- POST /send-message — отправить сообщение клиенту
- POST /health — проверка здоровья
"""

import logging
import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import Optional, List, Dict, Any
from aiogram import Bot
from aiogram.types import InlineKeyboardMarkup, InlineKeyboardButton
from aiogram.exceptions import TelegramForbiddenError, TelegramBadRequest

from bot.config import Config

logger = logging.getLogger(__name__)

app = FastAPI(title="Telegram Bot API Bridge", version="1.0.0")
bot_instance: Bot = None
config_instance: Config = None


class SendMessageRequest(BaseModel):
    chat_id: int
    text: str
    ticket_id: Optional[str] = None
    parse_mode: Optional[str] = "HTML"
    buttons: Optional[List[Dict[str, str]]] = None  # [{"text": "Да", "callback_data": "yes:123"}]


class SendMessageResponse(BaseModel):
    success: bool
    message_id: Optional[int] = None
    error: Optional[str] = None


@app.get("/health")
async def health():
    return {"status": "ok", "service": "telegram-bot-api-bridge"}


@app.post("/send-message", response_model=SendMessageResponse)
async def send_message(request: SendMessageRequest):
    """Отправка сообщения клиенту от имени бота"""
    if not bot_instance:
        raise HTTPException(status_code=503, detail="Bot not initialized")

    try:
        # Формируем клавиатуру с кнопками, если переданы
        reply_markup = None
        if request.buttons:
            keyboard = []
            row = []
            for btn in request.buttons:
                row.append(InlineKeyboardButton(
                    text=btn.get("text", ""),
                    callback_data=btn.get("callback_data", "")
                ))
            keyboard.append(row)
            reply_markup = InlineKeyboardMarkup(inline_keyboard=keyboard)

        # Если есть ticket_id, добавляем кнопки обратной связи автоматически
        if request.ticket_id and not request.buttons:
            reply_markup = InlineKeyboardMarkup(inline_keyboard=[
                [
                    InlineKeyboardButton(
                        text="✅ Проблема решена",
                        callback_data=f"resolved:{request.ticket_id}"
                    ),
                    InlineKeyboardButton(
                        text="👨‍💼 Нужна помощь оператора",
                        callback_data=f"escalate:{request.ticket_id}"
                    )
                ]
            ])

        sent = await bot_instance.send_message(
            chat_id=request.chat_id,
            text=request.text,
            parse_mode=request.parse_mode or "HTML",
            reply_markup=reply_markup
        )

        logger.info(f"Message sent to chat_id={request.chat_id}, message_id={sent.message_id}")
        return SendMessageResponse(success=True, message_id=sent.message_id)

    except TelegramForbiddenError:
        logger.warning(f"User {request.chat_id} blocked the bot")
        return SendMessageResponse(success=False, error="User blocked the bot")
    except TelegramBadRequest as e:
        logger.error(f"Bad request for chat_id={request.chat_id}: {e}")
        return SendMessageResponse(success=False, error=str(e))
    except Exception as e:
        logger.error(f"Failed to send message: {e}")
        return SendMessageResponse(success=False, error=str(e))


async def start_api_server(bot: Bot, config: Config):
    """Запускает FastAPI сервер в отдельном потоке"""
    global bot_instance, config_instance
    bot_instance = bot
    config_instance = config

    config_uvicorn = uvicorn.Config(
        app,
        host="0.0.0.0",
        port=8080,
        log_level="info"
    )
    server = uvicorn.Server(config_uvicorn)
    logger.info("Starting API bridge server on port 8080")
    await server.serve()