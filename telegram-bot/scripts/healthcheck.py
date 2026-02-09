#!/usr/bin/env python3
"""Проверка здоровья бота и зависимостей"""

import asyncio
import sys
from bot.services.api_client import APIClient
from bot.config import config

async def health_check():
    """Проверка всех компонентов"""
    print("🔍 Проверка здоровья системы...")
    
    # 1. Проверка конфигурации
    print("1. Конфигурация...", end=" ")
    if config.bot_token:
        print("✅")
    else:
        print("❌ BOT_TOKEN не установлен")
        return False
    
    # 2. Проверка local-proxy
    print("2. Local-proxy API...", end=" ")
    try:
        async with APIClient() as api:
            if await api.health_check():
                print("✅")
            else:
                print("❌")
                return False
    except Exception as e:
        print(f"❌ Ошибка: {e}")
        return False
    
    # 3. Проверка бота (получение информации)
    print("3. Telegram API...", end=" ")
    from aiogram import Bot
    bot = Bot(token=config.bot_token)
    try:
        me = await bot.get_me()
        print(f"✅ (@{me.username})")
    except Exception as e:
        print(f"❌ Ошибка: {e}")
        return False
    finally:
        await bot.session.close()
    
    print("\n🎉 Все проверки пройдены успешно!")
    return True

if __name__ == "__main__":
    success = asyncio.run(health_check())
    sys.exit(0 if success else 1)