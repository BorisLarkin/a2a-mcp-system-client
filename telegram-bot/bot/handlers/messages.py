"""
Обработчики входящих сообщений и callback'ов.
"""
import logging
import httpx
from aiogram import Router, Bot, F
from aiogram.types import Message, CallbackQuery, ReplyKeyboardMarkup, KeyboardButton
from aiogram.fsm.context import FSMContext
from aiogram.fsm.state import State, StatesGroup

from bot.config import config as cfg

logger = logging.getLogger(__name__)
router = Router()

# Глобальные переменные для бота
_bot: Bot = None


def set_bot(bot: Bot):
    global _bot
    _bot = bot


def set_config(c):
    global cfg
    cfg = c


class TicketStates(StatesGroup):
    waiting_for_description = State()


# ============ Обработчики сообщений ============

@router.message(F.text == "📝 Создать обращение")
async def create_ticket_prompt(message: Message, state: FSMContext):
    await state.set_state(TicketStates.waiting_for_description)
    await message.answer(
        "📝 Пожалуйста, опишите вашу проблему.\n"
        "Чем больше деталей, тем лучше!"
    )


@router.message(F.text == "📊 Мои обращения")
async def my_tickets(message: Message):
    chat_id = str(message.chat.id)
    
    try:
        async with httpx.AsyncClient(timeout=10.0) as client:
            resp = await client.get(
                f"{cfg.local_api_url}/api/v1/public/my-tickets",
                params={"client_external_id": chat_id},
                headers={"X-API-Key": cfg.api_key}
            )
            if resp.status_code == 200:
                data = resp.json()
                tickets = data.get("tickets", [])
                
                if not tickets:
                    await message.answer("У вас пока нет обращений.")
                    return
                
                text = "<b>Ваши обращения:</b>\n\n"
                for t in tickets[:10]:
                    status_map = {
                        "new": "🆕 Новое", "in_progress": "🔄 В работе",
                        "waiting": "⏳ Ожидает оператора", "waiting_for_feedback": "✅ Ждёт ответа",
                        "resolved": "✅ Решено", "closed": "🔒 Закрыто"
                    }
                    status_text = status_map.get(t.get("status", ""), "📝")
                    ticket_id = t.get("id", "")[:8]
                    original = (t.get("original_text", "") or "")[:50]
                    text += f"{status_text} <code>#{ticket_id}</code> — {original}\n"
                
                await message.answer(text)
            else:
                await message.answer("Не удалось загрузить обращения.")
    except Exception as e:
        logger.error(f"Failed to get tickets: {e}")
        await message.answer("Сервис временно недоступен.")


@router.message(TicketStates.waiting_for_description)
async def process_description(message: Message, state: FSMContext):
    await state.clear()
    await _create_ticket(message)


@router.message()
async def default_handler(message: Message, state: FSMContext):
    """Любое другое сообщение считаем обращением"""
    await state.clear()
    await _create_ticket(message)


async def _create_ticket(message: Message):
    """Отправка обращения в local-proxy"""
    await message.chat.do("typing")

    max_len = cfg.max_message_length if cfg else 4000
    api_url = cfg.local_api_url if cfg else "http://local-proxy:8080"
    api_key = cfg.api_key if cfg else ""

    ticket_data = {
        "text": message.text[:max_len],
        "client_external_id": str(message.chat.id),
        "channel_type": "telegram",
        "metadata": {
            "chat_id": message.chat.id,
            "username": message.from_user.username,
            "first_name": message.from_user.first_name,
            "last_name": message.from_user.last_name,
        }
    }

    try:
        async with httpx.AsyncClient(timeout=15.0) as client:
            resp = await client.post(
                f"{api_url}/api/v1/public/tickets",
                json=ticket_data,
                headers={"X-API-Key": api_key}
            )
            if resp.status_code == 201:
                data = resp.json()
                ticket_id = data.get("ticket_id", "unknown")
                await message.answer(
                    f"✅ Ваше обращение принято!\n"
                    f"Номер тикета: <code>{ticket_id[:8]}</code>\n\n"
                    f"⏳ Ожидайте ответа..."
                )
            else:
                await message.answer(
                    cfg.offline_response if cfg
                    else "Спасибо за обращение! Мы ответим при первой возможности."
                )
                logger.error(f"API returned status {resp.status_code}: {resp.text}")
    except Exception as e:
        logger.error(f"Failed to create ticket: {e}")
        offline = cfg.offline_response if cfg else "Спасибо за обращение!"
        await message.answer(offline)


# ============ Обработчики callback'ов (кнопки) ============

@router.callback_query(F.data.startswith("resolved:"))
async def callback_resolved(callback: CallbackQuery):
    ticket_id = callback.data.split(":", 1)[1]
    api_url = cfg.local_api_url if cfg else "http://local-proxy:8080"
    api_key = cfg.api_key if cfg else ""

    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            await client.put(
                f"{api_url}/api/v1/public/tickets/{ticket_id}/feedback",
                json={"status": "resolved", "feedback_status": "resolved"},
                headers={"X-API-Key": api_key}
            )
    except Exception as e:
        logger.error(f"Failed to update ticket {ticket_id}: {e}")

    try:
        await callback.message.edit_text(
            callback.message.text + "\n\n✅ <b>Спасибо за подтверждение!</b> Рады, что смогли помочь."
        )
    except Exception:
        pass
    await callback.answer()


@router.callback_query(F.data.startswith("escalate:"))
async def callback_escalate(callback: CallbackQuery):
    ticket_id = callback.data.split(":", 1)[1]
    api_url = cfg.local_api_url if cfg else "http://local-proxy:8080"
    api_key = cfg.api_key if cfg else ""

    try:
        async with httpx.AsyncClient(timeout=5.0) as client:
            await client.put(
                f"{api_url}/api/v1/public/tickets/{ticket_id}/feedback",
                json={"status": "waiting", "feedback_status": "escalate"},
                headers={"X-API-Key": api_key}
            )
    except Exception as e:
        logger.error(f"Failed to escalate ticket {ticket_id}: {e}")

    try:
        await callback.message.edit_text(
            callback.message.text + "\n\n👨‍💼 <b>Оператор уже уведомлён.</b> Ожидайте ответа."
        )
    except Exception:
        pass
    await callback.answer()