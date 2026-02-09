import asyncio
import signal
import sys
from typing import Optional
from contextlib import asynccontextmanager

from aiogram import Bot, Dispatcher
from aiogram.fsm.storage.memory import MemoryStorage
from aiogram.webhook.aiohttp_server import SimpleRequestHandler, setup_application

from bot.config import config
from bot.logger import logger
from bot.handlers import commands, messages, errors
from bot.handlers.base import UserActivityMiddleware, MaintenanceModeMiddleware
from bot.services.message_queue import message_queue


class TelegramBot:
    """Основной класс бота с расширенной функциональностью"""
    
    def __init__(self):
        self.bot: Optional[Bot] = None
        self.dp: Optional[Dispatcher] = None
        self.storage = MemoryStorage()
        
        # Очередь сообщений
        self.message_queue = message_queue
        
    @asynccontextmanager
    async def lifespan(self):
        """Контекстный менеджер для управления жизненным циклом"""
        # Startup
        logger.info("Starting Telegram bot...")
        
        # Инициализация бота
        self.bot = Bot(token=config.bot_token)
        self.dp = Dispatcher(storage=self.storage)
        
        # Настройка middleware
        self.dp.update.middleware(UserActivityMiddleware())
        self.dp.update.middleware(
            MaintenanceModeMiddleware(config.maintenance_mode)
        )
        
        # Регистрация обработчиков
        self.dp.include_router(commands.router)
        self.dp.include_router(messages.router)
        self.dp.include_router(errors.router)
        
        # Запуск фоновых задач
        background_tasks = []
        
        # Задача для retry failed сообщений
        retry_task = asyncio.create_task(
            self._retry_failed_messages_loop(),
            name="retry_failed_messages"
        )
        background_tasks.append(retry_task)
        
        # Задача для cleanup старых сообщений
        cleanup_task = asyncio.create_task(
            self._cleanup_old_messages_loop(),
            name="cleanup_old_messages"
        )
        background_tasks.append(cleanup_task)
        
        # Задача для мониторинга очереди
        monitor_task = asyncio.create_task(
            self._monitor_queue_loop(),
            name="monitor_queue"
        )
        background_tasks.append(monitor_task)
        
        yield
        
        # Shutdown
        logger.info("Shutting down bot...")
        
        # Отменяем фоновые задачи
        for task in background_tasks:
            task.cancel()
        
        # Ждем завершения задач
        await asyncio.gather(*background_tasks, return_exceptions=True)
        
        # Закрываем бота
        if self.bot:
            await self.bot.session.close()
        
        if self.dp:
            await self.dp.storage.close()
        
        logger.info("Bot shutdown complete")
    
    async def start_polling(self):
        """Запуск бота в режиме long-polling"""
        async with self.lifespan():
            logger.info("Bot started in polling mode. Press Ctrl+C to stop.")
            
            try:
                await self.dp.start_polling(
                    self.bot,
                    allowed_updates=["message", "callback_query"],
                    handle_signals=False  # Сами обрабатываем сигналы
                )
            except Exception as e:
                logger.error(f"Polling failed: {e}")
                raise
    
    async def start_webhook(self, webhook_url: str, host: str = "0.0.0.0", port: int = 8080):
        """Запуск бота в режиме webhook (если нужен)"""
        async with self.lifespan():
            from aiohttp import web
            
            # Настройка webhook
            await self.bot.set_webhook(
                url=webhook_url,
                secret_token=config.webhook_secret if hasattr(config, 'webhook_secret') else None
            )
            
            # Создание aiohttp приложения
            app = web.Application()
            webhook_requests_handler = SimpleRequestHandler(
                dispatcher=self.dp,
                bot=self.bot,
                secret_token=config.webhook_secret if hasattr(config, 'webhook_secret') else None
            )
            
            webhook_requests_handler.register(app, path=config.webhook_path)
            setup_application(app, self.dp, bot=self.bot)
            
            # Запуск сервера
            runner = web.AppRunner(app)
            await runner.setup()
            site = web.TCPSite(runner, host, port)
            
            logger.info(f"Webhook bot started on {host}:{port}")
            logger.info(f"Webhook URL: {webhook_url}")
            
            try:
                await site.start()
                await asyncio.Event().wait()  # Бесконечное ожидание
            except asyncio.CancelledError:
                pass
            finally:
                await runner.cleanup()
                await self.bot.session.close()
    
    # Фоновые задачи
    
    async def _retry_failed_messages_loop(self):
        """Цикл повторной отправки failed сообщений"""
        logger.info("Retry loop started")
        
        try:
            while True:
                await asyncio.sleep(60)  # Проверяем каждую минуту
                await self.message_queue.retry_failed_messages()
        except asyncio.CancelledError:
            logger.info("Retry loop stopped")
        except Exception as e:
            logger.error(f"Retry loop error: {e}")
    
    async def _cleanup_old_messages_loop(self):
        """Цикл очистки старых сообщений"""
        logger.info("Cleanup loop started")
        
        try:
            while True:
                await asyncio.sleep(3600)  # Проверяем каждый час
                await self.message_queue.cleanup_old_messages(days=7)
        except asyncio.CancelledError:
            logger.info("Cleanup loop stopped")
        except Exception as e:
            logger.error(f"Cleanup loop error: {e}")
    
    async def _monitor_queue_loop(self):
        """Цикл мониторинга очереди"""
        logger.info("Queue monitor started")
        
        try:
            while True:
                await asyncio.sleep(300)  # Проверяем каждые 5 минут
                
                stats = await self.message_queue.get_stats()
                
                if stats["queue_size"] > 100:
                    logger.warning(f"Queue is large: {stats['queue_size']} messages")
                
                if stats["failed"] > 10:
                    logger.warning(f"Many failed messages: {stats['failed']}")
                
                # Логируем статистику раз в час
                if datetime.now().minute == 0:  # Каждый час в 0 минут
                    logger.info(f"Queue stats: {stats}")
                    
        except asyncio.CancelledError:
            logger.info("Queue monitor stopped")
        except Exception as e:
            logger.error(f"Queue monitor error: {e}")
    
    async def stop(self):
        """Остановка бота"""
        logger.info("Stopping bot...")
        sys.exit(0)


async def main():
    """Точка входа"""
    bot = TelegramBot()
    
    # Обработка сигналов
    loop = asyncio.get_event_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, lambda: asyncio.create_task(bot.stop()))
    
    try:
        # Проверка конфигурации
        if not config.bot_token:
            logger.error("BOT_TOKEN not set in environment variables")
            sys.exit(1)
        
        # Запуск в режиме polling (для локального использования)
        await bot.start_polling()
        
    except KeyboardInterrupt:
        await bot.stop()
    except Exception as e:
        logger.error(f"Fatal error: {e}")
        await bot.stop()


if __name__ == "__main__":
    asyncio.run(main())