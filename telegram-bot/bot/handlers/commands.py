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