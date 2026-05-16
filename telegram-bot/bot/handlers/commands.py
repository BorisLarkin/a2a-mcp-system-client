"""
Обработчики команд бота.
"""
import logging
from aiogram import Router, F
from aiogram.types import Message, ReplyKeyboardMarkup, KeyboardButton
from aiogram.filters import Command, CommandStart

logger = logging.getLogger(__name__)
router = Router()


@router.message(CommandStart())
async def cmd_start(message: Message):
    keyboard = ReplyKeyboardMarkup(
        keyboard=[
            [KeyboardButton(text="📝 Создать обращение")],
            [KeyboardButton(text="📊 Мои обращения")],
        ],
        resize_keyboard=True
    )
    await message.answer(
        "👋 Здравствуйте! Я автоматический помощник поддержки.\n\n"
        "Опишите вашу проблему, и я постараюсь помочь.\n"
        "Или нажмите кнопку ниже.",
        reply_markup=keyboard
    )


@router.message(Command("help"))
async def cmd_help(message: Message):
    await message.answer(
        "<b>Помощь по использованию бота</b>\n\n"
        "1. Просто напишите вашу проблему в чат\n"
        "2. Бот автоматически создаст обращение\n"
        "3. Система подберёт решение и ответит вам\n"
        "4. Если решение не помогло — нажмите «Нужна помощь оператора»\n\n"
        "По любым вопросам обращайтесь к администратору."
    )


@router.message(Command("status"))
async def cmd_status(message: Message):
    await message.answer(
        "🟢 <b>Система работает</b>\n\n"
        "Бот активен и готов принимать обращения."
    )

@router.message(Command("my_tickets"))
async def cmd_my_tickets(message: Message):
    chat_id = str(message.chat.id)
    
    try:
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.get(
                f"{cfg.local_api_url}/api/v1/public/my-tickets",
                params={"client_external_id": chat_id},
                headers={"X-API-Key": cfg.api_key}
            )
            if resp.status_code == 200:
                data = resp.json()
                tickets = data.get("tickets", [])
                
                if not tickets:
                    await message.answer("У вас нет обращений.")
                    return
                
                text = "<b>Ваши обращения:</b>\n\n"
                for t in tickets[:10]:
                    status_emoji = {"new": "🆕", "in_progress": "🔄", "waiting": "⏳", 
                                   "waiting_for_feedback": "✅", "resolved": "✅", "closed": "🔒"}
                    emoji = status_emoji.get(t.get("status", ""), "📝")
                    ticket_id = t.get("id", "")[:8]
                    original = t.get("original_text", "")[:50]
                    text += f"{emoji} <code>#{ticket_id}</code> {original}\n"
                
                await message.answer(text)
            else:
                await message.answer("Не удалось загрузить обращения.")
    except Exception as e:
        logger.error(f"Failed to get tickets: {e}")
        await message.answer("Сервис временно недоступен.")