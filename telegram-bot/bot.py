import os
import logging
import asyncio
from typing import Dict, Optional
from dataclasses import dataclass
from datetime import datetime

import aiohttp
from aiogram import Bot, Dispatcher, types, Router
from aiogram.filters import Command
from aiogram.types import Message, ReplyKeyboardMarkup, KeyboardButton
from aiogram.client.default import DefaultBotProperties
import asyncpg
from pydantic import BaseModel

# Конфигурация
@dataclass
class Config:
    bot_token: str
    local_api_url: str = "http://localhost:8080"
    cloud_api_key: str = ""
    dispatcher_id: str = ""
    enable_debug: bool = True

class TicketRequest(BaseModel):
    text: str
    dispatcher_id: str
    channel: str = "telegram"
    metadata: Dict = {}

class BotService:
    def __init__(self, config: Config):
        self.config = config
        self.bot = Bot(
            token=config.bot_token,
            default=DefaultBotProperties(parse_mode="HTML")
        )
        self.dp = Dispatcher()
        self.router = Router()
        self.dp.include_router(self.router)
        
        # Регистрация обработчиков
        self.setup_handlers()
        
        # Сессия HTTP
        self.session: Optional[aiohttp.ClientSession] = None
        
        # Подключение к БД
        self.db_pool: Optional[asyncpg.Pool] = None
    
    def setup_handlers(self):
        @self.router.message(Command("start"))
        async def cmd_start(message: Message):
            keyboard = ReplyKeyboardMarkup(
                keyboard=[
                    [KeyboardButton(text="📝 Создать обращение")],
                    [KeyboardButton(text="📊 Статистика")],
                    [KeyboardButton(text="⚙️ Настройки")]
                ],
                resize_keyboard=True
            )
            
            welcome_text = """
🤖 <b>Диспетчерская поддержки MCP/A2A</b>

Я автоматический помощник для обработки обращений.

<b>Возможности:</b>
• Автоматическая классификация проблем
• Поиск решений в базе знаний
• Интеграция с интернет-поиском
• Офлайн-режим при потере связи

<b>Команды:</b>
/start - Главное меню
/help - Помощь
/status - Статус системы
/settings - Настройки
            """
            
            await message.answer(welcome_text, reply_markup=keyboard)
        
        @self.router.message(Command("status"))
        async def cmd_status(message: Message):
            # Проверяем доступность сервисов
            status_lines = []
            
            # Проверка локального прокси
            try:
                async with aiohttp.ClientSession() as session:
                    async with session.get(f"{self.config.local_api_url}/health") as resp:
                        local_status = "🟢" if resp.status == 200 else "🔴"
                status_lines.append(f"Локальный сервер: {local_status}")
            except:
                status_lines.append("Локальный сервер: 🔴")
            
            # Проверка облака
            try:
                async with aiohttp.ClientSession() as session:
                    async with session.get(
                        "https://api.mcp-system.com/health",
                        headers={"X-API-Key": self.config.cloud_api_key}
                    ) as resp:
                        cloud_status = "🟢" if resp.status == 200 else "🔴"
                status_lines.append(f"Облачный сервис: {cloud_status}")
            except:
                status_lines.append("Облачный сервис: 🔴")
            
            # Проверка БД
            try:
                await self.db_pool.execute("SELECT 1")
                db_status = "🟢"
            except:
                db_status = "🔴"
            status_lines.append(f"Локальная БД: {db_status}")
            
            status_text = "\n".join(status_lines)
            await message.answer(f"<b>Статус системы:</b>\n{status_text}")
        
        @self.router.message()
        async def handle_message(message: Message):
            # Игнорируем команды, которые уже обработаны
            if message.text.startswith('/'):
                return
            
            # Показываем индикатор набора
            await message.chat.do("typing")
            
            # Подготовка метаданных
            metadata = {
                "chat_id": message.chat.id,
                "username": message.from_user.username,
                "first_name": message.from_user.first_name,
                "last_name": message.from_user.last_name,
                "message_id": message.message_id,
                "timestamp": datetime.now().isoformat()
            }
            
            # Создание запроса
            ticket_req = TicketRequest(
                text=message.text,
                dispatcher_id=self.config.dispatcher_id,
                metadata=metadata
            )
            
            # Отправка в локальный прокси
            try:
                response = await self.send_to_local_api(ticket_req)
                
                # Отправляем ответ пользователю
                await message.answer(response["response"])
                
                # Если это офлайн-ответ, показываем уведомление
                if response.get("status") == "offline_pending":
                    offline_note = (
                        "\n\n<i>Примечание: Ответ сохранён локально и "
                        "будет отправлен в облако при восстановлении связи.</i>"
                    )
                    await message.answer(offline_note)
                    
            except Exception as e:
                logging.error(f"Error processing message: {e}")
                await message.answer(
                    "⚠️ Произошла ошибка при обработке запроса. "
                    "Попробуйте позже или свяжитесь с администратором."
                )
    
    async def send_to_local_api(self, ticket: TicketRequest) -> Dict:
        """Отправка запроса в локальный прокси-сервер"""
        if not self.session:
            self.session = aiohttp.ClientSession()
        
        async with self.session.post(
            f"{self.config.local_api_url}/api/v1/tickets",
            json=ticket.dict()
        ) as response:
            if response.status != 200:
                error_text = await response.text()
                raise Exception(f"API error {response.status}: {error_text}")
            
            return await response.json()
    
    async def init_db(self):
        """Инициализация подключения к БД"""
        self.db_pool = await asyncpg.create_pool(
            host="localhost",
            port=5432,
            user="mcp_client",
            password=os.getenv("DB_PASSWORD"),
            database="mcp_client_db",
            min_size=1,
            max_size=10
        )
    
    async def start(self):
        """Запуск бота"""
        await self.init_db()
        await self.dp.start_polling(self.bot)

def main():
    # Настройка логирования
    logging.basicConfig(level=logging.INFO)
    
    # Загрузка конфигурации
    config = Config(
        bot_token=os.getenv("TELEGRAM_BOT_TOKEN"),
        local_api_url=os.getenv("LOCAL_API_URL", "http://localhost:8080"),
        cloud_api_key=os.getenv("CLOUD_API_KEY"),
        dispatcher_id=os.getenv("DISPATCHER_ID"),
        enable_debug=os.getenv("DEBUG", "false").lower() == "true"
    )
    
    # Запуск бота
    bot_service = BotService(config)
    
    try:
        asyncio.run(bot_service.start())
    except KeyboardInterrupt:
        logging.info("Bot stopped")
    finally:
        # Закрытие сессий
        if bot_service.session:
            asyncio.run(bot_service.session.close())
        if bot_service.db_pool:
            asyncio.run(bot_service.db_pool.close())

if __name__ == "__main__":
    main()