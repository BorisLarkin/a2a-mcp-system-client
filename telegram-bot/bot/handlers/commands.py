from aiogram import Router, types
from aiogram.filters import Command
from bot.config import config
from bot.logger import logger

router = Router()

@router.message(Command("start"))
async def cmd_start(message: types.Message):
    """Обработчик команды /start"""
    welcome_text = f"""
👋 Здравствуйте! Это служба поддержки.

📞 *Как мы можем помочь?*
• Технические проблемы
• Вопросы по услугам
• Консультации

💡 *Просто напишите ваш вопрос*, и мы постараемся помочь как можно быстрее.

⚠️ *Пожалуйста, не отправляйте:*
• Конфиденциальные данные
• Пароли
• Банковские реквизиты
"""
    
    await message.answer(welcome_text, parse_mode="Markdown")
    logger.info(f"User {message.from_user.id} started the bot")

@router.message(Command("help"))
async def cmd_help(message: types.Message):
    """Обработчик команды /help"""
    help_text = """
📋 *Доступные команды:*
/start - Начало работы
/help - Эта справка
/status - Статус системы

🔧 *Как работает поддержка:*
1. Вы отправляете вопрос
2. Система анализирует его
3. Вы получаете ответ от AI или оператора
4. При необходимости оператор уточнит детали

⏱ *Время ответа:*
• AI ответ: 1-10 секунд
• Оператор: 1-10 минут (в рабочее время)

🔄 *Если долго нет ответа:*
Попробуйте переформулировать вопрос или отправьте его еще раз.
"""
    
    await message.answer(help_text, parse_mode="Markdown")

@router.message(Command("status"))
async def cmd_status(message: types.Message):
    """Обработчик команды /status"""
    from bot.services.api_client import APIClient
    
    status_text = "📊 *Статус системы:*\n\n"
    
    try:
        async with APIClient() as api_client:
            is_healthy = await api_client.health_check()
            
            if is_healthy:
                status_text += "✅ *Local-proxy:* Работает\n"
            else:
                status_text += "❌ *Local-proxy:* Недоступен\n"
                
    except Exception as e:
        logger.error(f"Health check failed: {e}")
        status_text += "❌ *Local-proxy:* Ошибка проверки\n"
    
    # Добавляем информацию о боте
    status_text += f"\n🤖 *Бот:* Работает\n"
    status_text += f"👤 *Пользователь ID:* `{message.from_user.id}`\n"
    
    if config.maintenance_mode:
        status_text += "\n⚠️ *Внимание:* Режим обслуживания"
    
    await message.answer(status_text, parse_mode="Markdown")