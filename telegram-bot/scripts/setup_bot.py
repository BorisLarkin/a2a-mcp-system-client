#!/usr/bin/env python3
"""Скрипт для настройки бота (webhook, команды и т.д.)"""

import asyncio
import sys
from aiogram import Bot
from bot.config import config

async def setup_bot():
    """Настройка бота"""
    bot = Bot(token=config.bot_token)
    
    try:
        # 1. Установка команд меню
        commands = [
            {"command": "start", "description": "Начать работу"},
            {"command": "help", "description": "Помощь"},
            {"command": "status", "description": "Статус системы"},
        ]
        
        await bot.set_my_commands(commands)
        print("✅ Команды меню установлены")
        
        # 2. Получение информации о боте
        me = await bot.get_me()
        print(f"🤖 Бот: @{me.username} ({me.full_name})")
        
        # 3. Проверка webhook (если используется)
        # webhook_info = await bot.get_webhook_info()
        # print(f"🌐 Webhook URL: {webhook_info.url}")
        
    finally:
        await bot.session.close()

if __name__ == "__main__":
    asyncio.run(setup_bot())