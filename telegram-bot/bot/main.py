import os
import logging
import asyncio
import httpx
from aiogram import Bot, Dispatcher
from aiogram.client.default import DefaultBotProperties
from aiogram.enums import ParseMode
from aiogram.fsm.storage.memory import MemoryStorage

from bot.config import Config
from bot.handlers import commands, messages, errors
from bot.api_server import start_api_server

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


async def fetch_api_key(local_api_url: str) -> tuple:
    try:
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.get(
                f"{local_api_url}/api/v1/public/bot-key",
                headers={"X-Setup-Secret": "internal_setup_secret_123"}
            )
            if resp.status_code == 200:
                data = resp.json()
                return data.get("api_key", ""), data.get("dispatcher_id", "")
    except Exception as e:
        logger.error(f"Failed to fetch API key: {e}")
    return "", ""


async def main():
    config = Config(
        bot_token=os.getenv("TELEGRAM_BOT_TOKEN", ""),
        local_api_url=os.getenv("LOCAL_API_URL", "http://localhost:3000"),
        api_key=os.getenv("API_KEY", ""),
        dispatcher_id=os.getenv("DISPATCHER_ID", ""),
        bot_username=os.getenv("BOT_USERNAME", "a2a_mcp_company_support_bot"),
        offline_response=os.getenv("OFFLINE_RESPONSE", "Спасибо за обращение! Мы получили ваше сообщение и ответим при первой возможности."),
        rate_limit_per_user=int(os.getenv("RATE_LIMIT_PER_USER", "5")),
        max_message_length=int(os.getenv("MAX_MESSAGE_LENGTH", "4000")),
        maintenance_mode=os.getenv("MAINTENANCE_MODE", "false").lower() == "true",
    )

    api_key, dispatcher_id = await fetch_api_key(config.local_api_url)
    if api_key:
        config.api_key = api_key
        config.dispatcher_id = dispatcher_id
        logger.info(f"Bot API key fetched from local-proxy")
    else:
        logger.warning("Could not fetch API key, using default from .env")

    if not config.bot_token:
        logger.error("TELEGRAM_BOT_TOKEN not set!")
        return

    bot = Bot(
        token=config.bot_token,
        default=DefaultBotProperties(parse_mode=ParseMode.HTML)
    )
    dp = Dispatcher(storage=MemoryStorage())

    dp.include_router(commands.router)
    dp.include_router(messages.router)
    dp.include_router(errors.router)

    import bot.config as config_module
    config_module.config = config
    messages.set_config(config)
    messages.set_bot(bot)

    api_task = asyncio.create_task(start_api_server(bot, config))

    logger.info("Starting bot polling...")
    try:
        await dp.start_polling(bot)
    finally:
        api_task.cancel()
        await bot.session.close()
        logger.info("Bot stopped")


if __name__ == "__main__":
    asyncio.run(main())