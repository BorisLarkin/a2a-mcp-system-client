from aiogram import Router, types, F
from aiogram.filters import StateFilter
from aiogram.fsm.context import FSMContext
from aiogram.fsm.state import State, StatesGroup
from typing import Optional

from bot.config import config
from bot.logger import logger
from bot.services.api_client import APIClient
from bot.services.rate_limiter import RateLimiter
from bot.models.ticket import TicketCreate
from bot.utils.validators import validate_message_length

router = Router()
rate_limiter = RateLimiter()

class SupportStates(StatesGroup):
    """Состояния для поддержки"""
    waiting_for_response = State()

@router.message(F.text)
async def handle_message(message: types.Message, state: FSMContext):
    """Основной обработчик текстовых сообщений"""
    
    user_id = message.from_user.id
    chat_id = message.chat.id
    text = message.text
    
    # Проверка длины сообщения
    if not validate_message_length(text, config.max_message_length):
        await message.answer(
            f"❌ Сообщение слишком длинное. Максимум {config.max_message_length} символов."
        )
        return
    
    # Проверка rate limit
    if not rate_limiter.is_allowed(user_id):
        await message.answer(
            "⏳ Слишком много запросов. Пожалуйста, подождите 1 минуту."
        )
        return
    
    # Режим обслуживания
    if config.maintenance_mode:
        await message.answer(
            "🔧 Система на техническом обслуживании. "
            "Пожалуйста, повторите попытку позже."
        )
        return
    
    # Показываем статус обработки
    status_msg = await message.answer("⏳ Обрабатываю ваш запрос...")
    
    try:
        # Создаем модель тикета
        from uuid import uuid4  # ДОБАВЛЯЕМ ИМПОРТ
        
        ticket_data = TicketCreate(
            client_identifier=str(chat_id),
            client_name=message.from_user.full_name,
            initial_message=text,
            metadata={
                "message_id": message.message_id,
                "user_id": user_id,
                "username": message.from_user.username,
                "is_bot": message.from_user.is_bot,
                "language_code": message.from_user.language_code,
                "chat_type": message.chat.type,
            }
        )
        
        # Отправляем в local-proxy
        async with APIClient() as api_client:
            response = await api_client.create_ticket(ticket_data)
            
            # Удаляем статус сообщение
            try:
                await status_msg.delete()
            except:
                pass
            
            # Обрабатываем ответ
            if response.status == "failed":
                # Ошибка API
                logger.error(f"API error for user {user_id}: {response.error}")
                
                # Отправляем офлайн-ответ
                await message.answer(config.offline_response)
                
                # Сохраняем для повторной отправки
                await save_for_retry(ticket_data)
                
            elif response.final_response:
                # Есть готовый ответ (от AI или оператора)
                await message.answer(response.final_response)
                logger.info(f"Response sent to user {user_id}")
                
            elif response.status == "processing":
                # Обработка в процессе
                await message.answer(
                    "✅ Ваш вопрос принят в обработку. "
                    "Ожидайте ответа от оператора."
                )
                
                # Устанавливаем состояние ожидания
                await state.set_state(SupportStates.waiting_for_response)
                await state.update_data(ticket_id=response.ticket_id)
                
            else:
                # Неизвестный статус
                await message.answer(
                    "Получен ваш вопрос. Ожидайте ответа."
                )
                
    except Exception as e:
        logger.error(f"Error processing message from {user_id}: {e}")
        
        # Удаляем статус сообщение при ошибке
        try:
            await status_msg.delete()
        except:
            pass
        
        # Отправляем fallback ответ
        await message.answer(config.offline_response)
        
        # Сохраняем для повторной отправки
        await save_for_retry(TicketCreate(
            client_identifier=str(chat_id),
            client_name=message.from_user.full_name,
            initial_message=text,
            metadata={"error": str(e)}
        ))

async def save_for_retry(ticket_data: TicketCreate):
    """Сохранить тикет для повторной отправки"""
    # В простейшем случае - логируем
    logger.warning(f"Ticket saved for retry: {ticket_data.client_identifier}")
    
    # В прод версии можно сохранить в Redis или файл
    if config.redis_enabled:
        # Реализация с Redis
        pass

@router.message(F.text, StateFilter(SupportStates.waiting_for_response))
async def handle_followup_message(message: types.Message, state: FSMContext):
    """Обработка дополнительных сообщений пока ждем ответ"""
    
    # Пользователь может отправлять уточнения пока ждем ответ
    await message.answer(
        "💬 Получил ваше уточнение. "
        "Оператор увидит все сообщения когда возьмет ваш вопрос."
    )
    
    # Сохраняем уточнение в local-proxy
    data = await state.get_data()
    ticket_id = data.get("ticket_id")
    
    if ticket_id:
        async with APIClient() as api_client:
            await api_client._make_request(
                "POST",
                f"{config.api_endpoint}/{ticket_id}/followup",
                json={
                    "message": message.text,
                    "message_id": message.message_id
                }
            )