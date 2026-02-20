# ~/git/a2a-mcp-system-client/telegram-bot/bot.py
import os
import logging
import asyncio
import json
from typing import Dict, Optional, Any
from dataclasses import dataclass, asdict
from datetime import datetime
from enum import Enum

import aiohttp
from aiogram import Bot, Dispatcher, types, F
from aiogram.filters import Command, CommandStart
from aiogram.types import Message, ReplyKeyboardMarkup, KeyboardButton
from aiogram.client.default import DefaultBotProperties
from aiogram.enums import ParseMode
from aiogram.utils.keyboard import ReplyKeyboardBuilder
from aiogram.fsm.context import FSMContext
from aiogram.fsm.state import State, StatesGroup
from aiogram.fsm.storage.memory import MemoryStorage

# Настройка логирования
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class ChannelType(Enum):
    TELEGRAM = "telegram"
    EMAIL = "email"
    WEB = "web"


class TicketStatus(Enum):
    NEW = "new"
    PROCESSING = "processing"
    WAITING = "waiting"
    RESOLVED = "resolved"
    CLOSED = "closed"


@dataclass
class Config:
    """Конфигурация бота"""
    bot_token: str
    local_api_url: str = "http://localhost:8080"
    cloud_api_key: str = ""
    dispatcher_id: str = ""
    debug: bool = False
    db_password: str = ""
    db_host: str = "mcp-client-db"
    db_port: int = 5432
    db_user: str = "mcp_client"
    db_name: str = "mcp_client_db"


@dataclass
class TicketRequest:
    """Запрос на создание тикета"""
    text: str
    dispatcher_id: str
    channel: str = "telegram"
    priority: str = "medium"
    subject: Optional[str] = None
    metadata: Dict[str, Any] = None
    
    def to_dict(self):
        result = asdict(self)
        if result["metadata"] is None:
            result["metadata"] = {}
        return result


@dataclass
class TicketResponse:
    """Ответ от API при создании тикета"""
    ticket_id: str
    status: str
    response: Optional[str] = None
    ai_analysis: Optional[Dict] = None
    queue_position: Optional[int] = None


# Состояния FSM
class TicketStates(StatesGroup):
    waiting_for_description = State()
    waiting_for_confirmation = State()
    waiting_for_feedback = State()


class BotService:
    """Основной сервис бота"""
    
    def __init__(self, config: Config):
        self.config = config
        self.storage = MemoryStorage()
        
        # Инициализация бота и диспетчера
        self.bot = Bot(
            token=config.bot_token,
            default=DefaultBotProperties(
                parse_mode=ParseMode.HTML
            )
        )
        self.dp = Dispatcher(storage=self.storage)
        
        # HTTP сессия для API
        self.session: Optional[aiohttp.ClientSession] = None
        
        # Очередь для офлайн-режима
        self.offline_queue: asyncio.Queue = asyncio.Queue()
        self.online_mode = True
        self.retry_task: Optional[asyncio.Task] = None
        
        logger.info(f"Bot initialized with API URL: {config.local_api_url}")
        
    async def start(self):
        """Запуск бота"""
        # Создание HTTP сессии
        self.session = aiohttp.ClientSession()
        
        # Регистрация обработчиков
        self._register_handlers()
        
        # Запуск фоновых задач
        self.retry_task = asyncio.create_task(self._process_offline_queue())
        
        logger.info("Starting bot polling...")
        await self.dp.start_polling(self.bot)
    
    async def stop(self):
        """Остановка бота"""
        if self.retry_task:
            self.retry_task.cancel()
        
        if self.session:
            await self.session.close()
        
        if self.storage:
            await self.storage.close()
        
        await self.bot.session.close()
        logger.info("Bot stopped")
    
    def _register_handlers(self):
        """Регистрация всех обработчиков"""
        
        # Команды
        self.dp.message.register(self.cmd_start, CommandStart())
        self.dp.message.register(self.cmd_help, Command("help"))
        self.dp.message.register(self.cmd_status, Command("status"))
        self.dp.message.register(self.cmd_settings, Command("settings"))
        
        # Кнопки меню
        self.dp.message.register(self.create_ticket_handler, F.text == "📝 Создать обращение")
        self.dp.message.register(self.stats_handler, F.text == "📊 Статистика")
        self.dp.message.register(self.settings_handler, F.text == "⚙️ Настройки")
        
        # FSM обработчики
        self.dp.message.register(self.process_description, TicketStates.waiting_for_description)
        self.dp.message.register(self.process_confirmation, TicketStates.waiting_for_confirmation)
        
        # Обработчик по умолчанию
        self.dp.message.register(self.default_message_handler)
    
    async def cmd_start(self, message: Message):
        """Обработчик команды /start"""
        builder = ReplyKeyboardBuilder()
        builder.add(
            KeyboardButton(text="📝 Создать обращение"),
            KeyboardButton(text="📊 Статистика"),
            KeyboardButton(text="⚙️ Настройки")
        )
        builder.adjust(1)
        
        welcome_text = (
            "🤖 <b>Диспетчерская поддержки MCP/A2A</b>\n\n"
            "Я автоматический помощник для обработки обращений.\n\n"
            "<b>Возможности:</b>\n"
            "• Автоматическая классификация проблем\n"
            "• Поиск решений в базе знаний\n"
            "• Интеграция с интернет-поиском\n"
            "• Офлайн-режим при потере связи\n\n"
            "<b>Команды:</b>\n"
            "/start - Главное меню\n"
            "/help - Помощь\n"
            "/status - Статус системы\n"
            "/settings - Настройки"
        )
        
        await message.answer(
            welcome_text,
            reply_markup=builder.as_markup(resize_keyboard=True)
        )
    
    async def cmd_help(self, message: Message):
        """Обработчик команды /help"""
        help_text = (
            "<b>Помощь по использованию бота</b>\n\n"
            "1. Нажмите <b>«Создать обращение»</b> для нового запроса\n"
            "2. Опишите вашу проблему подробно\n"
            "3. Бот автоматически классифицирует запрос\n"
            "4. Получите ответ от системы поддержки\n\n"
            "Если у вас возникли проблемы, обратитесь к администратору."
        )
        await message.answer(help_text)
    
    async def cmd_status(self, message: Message):
        """Обработчик команды /status"""
        await message.chat.do("typing")
        
        status_lines = []
        
        # Проверка локального прокси
        try:
            async with self.session.get(
                f"{self.config.local_api_url}/health",
                timeout=5
            ) as resp:
                local_status = "🟢 Работает" if resp.status == 200 else "🔴 Недоступен"
                if resp.status == 200:
                    data = await resp.json()
                    status_lines.append(f"🖥️ Локальный сервер: {local_status}")
                    status_lines.append(f"⏱️ Время ответа: {data.get('time', 'N/A')}")
                else:
                    status_lines.append(f"🖥️ Локальный сервер: {local_status}")
        except Exception as e:
            status_lines.append(f"🖥️ Локальный сервер: 🔴 Ошибка подключения")
            logger.error(f"Health check failed: {e}")
        
        # Проверка облака
        if self.config.cloud_api_key:
            try:
                async with self.session.get(
                    "https://api.mcp-system.com/health",
                    headers={"X-API-Key": self.config.cloud_api_key},
                    timeout=5
                ) as resp:
                    cloud_status = "🟢 Доступен" if resp.status == 200 else "🔴 Недоступен"
                    status_lines.append(f"☁️ Облачный сервис: {cloud_status}")
            except:
                status_lines.append(f"☁️ Облачный сервис: 🔴 Недоступен")
        
        # Режим работы
        mode_status = "🟢 Онлайн" if self.online_mode else "🟡 Офлайн (буферизация)"
        status_lines.append(f"📡 Режим работы: {mode_status}")
        
        # Размер очереди
        if not self.online_mode:
            queue_size = self.offline_queue.qsize()
            status_lines.append(f"📦 Ожидает отправки: {queue_size}")
        
        status_text = "\n".join(status_lines)
        await message.answer(f"<b>Статус системы:</b>\n{status_text}")
    
    async def cmd_settings(self, message: Message):
        """Обработчик команды /settings"""
        settings_text = (
            "<b>Настройки</b>\n\n"
            "Здесь будут настройки уведомлений и предпочтений.\n\n"
            "<i>Функция в разработке</i>"
        )
        await message.answer(settings_text)
    
    async def create_ticket_handler(self, message: Message, state: FSMContext):
        """Начало создания обращения"""
        await state.set_state(TicketStates.waiting_for_description)
        await message.answer(
            "📝 Пожалуйста, опишите вашу проблему подробно.\n"
            "Чем больше деталей, тем быстрее мы сможем помочь!\n\n"
            "<i>Напишите /cancel для отмены</i>"
        )
    
    async def process_description(self, message: Message, state: FSMContext):
        """Обработка описания проблемы"""
        if message.text == "/cancel":
            await state.clear()
            await message.answer("❌ Создание обращения отменено.")
            return
        
        # Сохраняем описание
        await state.update_data(description=message.text)
        await state.set_state(TicketStates.waiting_for_confirmation)
        
        # Показываем предпросмотр
        preview = (
            f"<b>Предпросмотр обращения:</b>\n\n"
            f"{message.text[:200]}{'...' if len(message.text) > 200 else ''}\n\n"
            f"Отправить? (Да/Нет)"
        )
        
        await message.answer(preview)
    
    async def process_confirmation(self, message: Message, state: FSMContext):
        """Подтверждение отправки"""
        if message.text.lower() in ["да", "yes", "ok", "✅"]:
            await self._submit_ticket(message, state)
        else:
            await state.clear()
            await message.answer("❌ Отправка отменена. Вы можете создать новое обращение.")
    
    async def _submit_ticket(self, message: Message, state: FSMContext):
        """Отправка тикета в API"""
        await message.chat.do("typing")
        
        data = await state.get_data()
        description = data.get("description", "")
        
        # Подготовка метаданных
        metadata = {
            "chat_id": message.chat.id,
            "username": message.from_user.username,
            "first_name": message.from_user.first_name,
            "last_name": message.from_user.last_name,
            "message_id": message.message_id,
            "timestamp": datetime.now().isoformat(),
            "language": message.from_user.language_code
        }
        
        ticket = TicketRequest(
            text=description,
            dispatcher_id=self.config.dispatcher_id or "default",
            metadata=metadata,
            subject=description[:100]  # Первые 100 символов как тема
        )
        
        # Отправка в API
        response = await self._send_ticket(ticket)
        
        if response:
            await message.answer(
                f"✅ <b>Обращение #{response.ticket_id[:8]} создано!</b>\n\n"
                f"{response.response or 'Специалист скоро ответит.'}\n\n"
                f"Статус: {response.status}"
            )
            
            # Если есть AI анализ, показываем
            if response.ai_analysis:
                analysis_text = (
                    f"📊 <b>Анализ запроса:</b>\n"
                    f"• Категория: {response.ai_analysis.get('category', 'Не определена')}\n"
                    f"• Приоритет: {response.ai_analysis.get('priority', 'Средний')}\n"
                )
                await message.answer(analysis_text)
        else:
            # Офлайн режим - сохраняем в очередь
            await self.offline_queue.put((ticket, message.chat.id))
            await message.answer(
                "⚠️ Сервис временно недоступен.\n"
                "Ваше обращение сохранено и будет отправлено автоматически "
                "при восстановлении связи."
            )
        
        await state.clear()
    
    async def _send_ticket(self, ticket: TicketRequest) -> Optional[TicketResponse]:
        """Отправка тикета в API"""
        try:
            async with self.session.post(
                f"{self.config.local_api_url}/api/v1/tickets",
                json=ticket.to_dict(),
                timeout=10
            ) as response:
                if response.status == 201:
                    data = await response.json()
                    self.online_mode = True
                    return TicketResponse(
                        ticket_id=data.get("ticket_id", "unknown"),
                        status=data.get("status", "created"),
                        response=data.get("response"),
                        ai_analysis=data.get("ai_analysis"),
                        queue_position=data.get("queue_position")
                    )
                elif response.status == 503:
                    # Сервис недоступен - переходим в офлайн режим
                    self.online_mode = False
                    logger.warning("API unavailable, switching to offline mode")
                    return None
                else:
                    error_text = await response.text()
                    logger.error(f"API error {response.status}: {error_text}")
                    return None
                    
        except asyncio.TimeoutError:
            logger.error("API request timeout")
            self.online_mode = False
            return None
        except aiohttp.ClientConnectorError as e:
            logger.error(f"Connection error: {e}")
            self.online_mode = False
            return None
        except Exception as e:
            logger.error(f"Unexpected error: {e}")
            return None
    
    async def _process_offline_queue(self):
        """Фоновая обработка очереди офлайн-сообщений"""
        while True:
            try:
                if self.online_mode and not self.offline_queue.empty():
                    # Пытаемся отправить накопленные сообщения
                    processed = 0
                    while not self.offline_queue.empty() and processed < 10:
                        ticket, chat_id = await self.offline_queue.get()
                        
                        response = await self._send_ticket(ticket)
                        if response:
                            try:
                                await self.bot.send_message(
                                    chat_id,
                                    f"✅ Ваше обращение было успешно отправлено!\n"
                                    f"Номер тикета: #{response.ticket_id[:8]}"
                                )
                            except Exception as e:
                                logger.error(f"Failed to notify user {chat_id}: {e}")
                            processed += 1
                        else:
                            # Не удалось отправить - возвращаем в очередь
                            await self.offline_queue.put((ticket, chat_id))
                            break
                            
                        await asyncio.sleep(1)  # Задержка между отправками
                
                await asyncio.sleep(5)  # Проверка каждые 5 секунд
                
            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.error(f"Queue processing error: {e}")
                await asyncio.sleep(10)
    
    async def stats_handler(self, message: Message):
        """Обработчик статистики"""
        await message.chat.do("typing")
        
        # Здесь можно получить статистику из API
        stats_text = (
            "📊 <b>Статистика</b>\n\n"
            "• Всего обращений: Загрузка...\n"
            "• В работе: ...\n"
            "• Решено: ...\n"
            "• Среднее время ответа: ...\n\n"
            "<i>Статистика обновляется</i>"
        )
        
        await message.answer(stats_text)
    
    async def settings_handler(self, message: Message):
        """Обработчик настроек"""
        await message.answer(
            "⚙️ <b>Настройки</b>\n\n"
            "1. Уведомления\n"
            "2. Язык интерфейса\n"
            "3. Приоритет по умолчанию\n\n"
            "<i>Выберите пункт меню</i>"
        )
    
    async def default_message_handler(self, message: Message):
        """Обработчик по умолчанию"""
        # Если пользователь просто пишет сообщение, предлагаем создать обращение
        keyboard = ReplyKeyboardMarkup(
            keyboard=[[KeyboardButton(text="📝 Создать обращение")]],
            resize_keyboard=True
        )
        
        await message.answer(
            "Я не распознал команду. Хотите создать обращение в поддержку?",
            reply_markup=keyboard
        )


async def main():
    """Точка входа"""
    # Загрузка конфигурации из переменных окружения
    config = Config(
        bot_token=os.getenv("TELEGRAM_BOT_TOKEN", ""),
        local_api_url=os.getenv("LOCAL_API_URL", "http://local-proxy:8080"),
        cloud_api_key=os.getenv("CLOUD_API_KEY", ""),
        dispatcher_id=os.getenv("DISPATCHER_ID", ""),
        debug=os.getenv("DEBUG", "false").lower() == "true",
        db_password=os.getenv("DB_PASSWORD", ""),
        db_host=os.getenv("DB_HOST", "mcp-client-db"),
        db_user=os.getenv("DB_USER", "mcp_client"),
        db_name=os.getenv("DB_NAME", "mcp_client_db")
    )
    
    if not config.bot_token:
        logger.error("TELEGRAM_BOT_TOKEN not set!")
        return
    
    bot_service = BotService(config)
    
    try:
        await bot_service.start()
    except KeyboardInterrupt:
        logger.info("Received stop signal")
    finally:
        await bot_service.stop()


if __name__ == "__main__":
    asyncio.run(main())