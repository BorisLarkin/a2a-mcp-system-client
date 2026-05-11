import os
import logging
import asyncio
from aiogram import Bot, Dispatcher
from aiogram.client.default import DefaultBotProperties
from aiogram.enums import ParseMode
from aiogram.fsm.storage.memory import MemoryStorage

from bot.config import Config, config as cfg_module
from bot.handlers import commands, messages, errors
from bot.api_server import start_api_server


logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


async def main():
    config = Config(
        bot_token=os.getenv("TELEGRAM_BOT_TOKEN", ""),
        local_api_url=os.getenv("LOCAL_API_URL", "http://local-proxy:8080"),
        api_key=os.getenv("API_KEY", ""),
        dispatcher_id=os.getenv("DISPATCHER_ID", ""),
        bot_username=os.getenv("BOT_USERNAME", "a2a_mcp_company_support_bot"),
        offline_response=os.getenv("OFFLINE_RESPONSE", "Спасибо за обращение! Мы получили ваше сообщение и ответим при первой возможности."),
        rate_limit_per_user=int(os.getenv("RATE_LIMIT_PER_USER", "5")),
        max_message_length=int(os.getenv("MAX_MESSAGE_LENGTH", "4000")),
        maintenance_mode=os.getenv("MAINTENANCE_MODE", "false").lower() == "true",
    )
    
    if not config.bot_token:
        logger.error("TELEGRAM_BOT_TOKEN not set!")
        return

    # Инициализация бота
    bot = Bot(
        token=config.bot_token,
        default=DefaultBotProperties(parse_mode=ParseMode.HTML)
    )
    dp = Dispatcher(storage=MemoryStorage())

    # Регистрация обработчиков
    dp.include_router(commands.router)
    dp.include_router(messages.router)
    dp.include_router(errors.router)

    # Передаём config и bot в handlers
    import bot.config as config_module
    config_module.config = config
    messages.set_config(config)
    messages.set_bot(bot)

    # Запуск API-сервера для приёма команд от local-proxy
    api_task = asyncio.create_task(start_api_server(bot, config))

    # Запуск бота
    logger.info("Starting bot polling...")
    try:
        await dp.start_polling(bot)
    finally:
        api_task.cancel()
        await bot.session.close()
        logger.info("Bot stopped")


if __name__ == "__main__":
    asyncio.run(main())