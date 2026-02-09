import pytest
from unittest.mock import AsyncMock, MagicMock
from aiogram.types import Message, User, Chat
from bot.handlers.messages import handle_message

@pytest.mark.asyncio
async def test_handle_message():
    """Тест обработки сообщения"""
    
    # Создаем mock объекты
    message = AsyncMock(spec=Message)
    message.text = "Тестовое сообщение"
    message.from_user = User(
        id=123,
        is_bot=False,
        first_name="Test",
        last_name="User"
    )
    message.chat = Chat(id=123, type="private")
    
    state = AsyncMock(spec=FSMContext)
    
    # Вызываем обработчик
    await handle_message(message, state)
    
    # Проверяем, что сообщение было обработано
    assert message.answer.called

@pytest.mark.asyncio
async def test_rate_limit():
    """Тест ограничения запросов"""
    from bot.services.rate_limiter import RateLimiter
    
    limiter = RateLimiter()
    user_id = 123
    
    # Первые 5 запросов должны проходить
    for i in range(5):
        assert limiter.is_allowed(user_id) == True
    
    # 6-й запрос должен быть отклонен
    assert limiter.is_allowed(user_id) == False