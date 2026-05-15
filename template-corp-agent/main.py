import os
import json
import logging
import uuid
from datetime import datetime
from fastapi import FastAPI, Request
from pydantic import BaseModel
from typing import Dict, Any, List, Optional
import httpx

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="Corporate Agent", version="1.0.0")

# --- Конфигурация ---
AGENT_NAME = os.getenv("AGENT_NAME", "corporate-agent-v1")
AGENT_TYPE = os.getenv("AGENT_TYPE", "corporate")
DB_API_URL = os.getenv("DB_API_URL", "http://local-proxy:8080")

# Мок-данные о компании
COMPANY_INFO = {
    "name": "ООО «ТехПоддержка Плюс»",
    "description": "Интернет-провайдер, работающий с клиентами в Москве и области с 2015 года.",
    "contacts": {
        "phone": "+7 (495) 123-45-67",
        "email": "support@techplus.ru",
        "address": "г. Москва, ул. Тверская, д. 1",
        "working_hours": "Пн-Пт 09:00-21:00, Сб-Вс 10:00-18:00"
    },
    "services": [
        "Подключение домашнего интернета",
        "Настройка Wi-Fi оборудования",
        "Корпоративные тарифы",
        "Выделенные линии связи",
        "IPTV и видеонаблюдение"
    ],
    "tariffs": [
        {"name": "Старт", "speed": "100 Мбит/с", "price": "499 ₽/мес"},
        {"name": "Комфорт", "speed": "300 Мбит/с", "price": "699 ₽/мес"},
        {"name": "Бизнес", "speed": "1 Гбит/с", "price": "1499 ₽/мес"}
    ]
}

# Мок-статусы заявок
MOCK_TICKETS = {
    "123": {"status": "В обработке", "operator": "Иванов А.А.", "deadline": "2026-05-20"},
    "456": {"status": "Выполнена", "operator": "Петров Б.Б.", "deadline": "2026-05-15"},
    "789": {"status": "Требует уточнения", "operator": "Сидоров В.В.", "deadline": "2026-05-18"},
}

# --- A2A Models ---
class A2ATaskRequest(BaseModel):
    skill_id: str
    input: Dict[str, Any]

class A2ATaskResponse(BaseModel):
    task_id: Optional[str] = None
    status: str
    output: Optional[Dict[str, Any]] = None
    error: Optional[str] = None


# ============ Skills ============

def skill_company_info(query: str = None) -> dict:
    """Информация о компании"""
    if query and "тариф" in query.lower():
        return {
            "type": "tariffs",
            "data": COMPANY_INFO["tariffs"],
            "message": "Доступные тарифы компании"
        }
    elif query and ("контакт" in query.lower() or "телефон" in query.lower() or "адрес" in query.lower()):
        return {
            "type": "contacts",
            "data": COMPANY_INFO["contacts"],
            "message": "Контактная информация"
        }
    elif query and ("услуг" in query.lower() or "сервис" in query.lower()):
        return {
            "type": "services",
            "data": COMPANY_INFO["services"],
            "message": "Список услуг"
        }
    else:
        return {
            "type": "general",
            "data": COMPANY_INFO,
            "message": f"Общая информация о компании {COMPANY_INFO['name']}"
        }


def skill_ticket_status(ticket_id: str = None) -> dict:
    """Статус заявки"""
    if not ticket_id:
        return {"error": "Не указан номер заявки", "available_examples": list(MOCK_TICKETS.keys())}
    
    ticket = MOCK_TICKETS.get(ticket_id)
    if not ticket:
        return {"error": f"Заявка #{ticket_id} не найдена"}
    
    return {
        "ticket_id": ticket_id,
        "status": ticket["status"],
        "operator": ticket["operator"],
        "deadline": ticket["deadline"],
        "checked_at": datetime.now().isoformat()
    }


def skill_check_balance(account_id: str = None) -> dict:
    """Проверка баланса (заглушка)"""
    return {
        "account_id": account_id or "unknown",
        "balance": 1250.50,
        "currency": "₽",
        "next_payment": "2026-06-01",
        "status": "Активен"
    }


# ============ API Endpoints ============

@app.get("/health")
async def health():
    return {
        "status": "healthy",
        "agent": AGENT_NAME,
        "type": AGENT_TYPE,
        "timestamp": datetime.now().isoformat()
    }


@app.get("/.well-known/agent.json")
async def discovery():
    return {
        "name": AGENT_NAME,
        "type": AGENT_TYPE,
        "description": "Корпоративный агент — информация о компании, статусы заявок, проверка баланса",
        "version": "1.0.0",
        "endpoint": f"http://{AGENT_TYPE}:9004",
        "capabilities": ["corporate", "company_info", "ticket_status"],
        "skills": [
            {
                "id": "company_info",
                "description": "Получить информацию о компании, тарифах, услугах, контактах",
                "input_schema": {
                    "type": "object",
                    "properties": {
                        "query": {"type": "string", "description": "Что ищем (тарифы, контакты, услуги)"},
                        "text": {"type": "string", "description": "Текст обращения (альтернативное поле)"},
                        "previous_results": {"type": "object", "description": "Результаты предыдущих шагов"}
                    }
                },
                "output_schema": {
                    "type": "object",
                    "properties": {
                        "type": {"type": "string"},
                        "data": {"type": "object"},
                        "message": {"type": "string"}
                    }
                }
            },
            {
                "id": "ticket_status",
                "description": "Проверить статус заявки по номеру",
                "input_schema": {
                    "type": "object",
                    "properties": {
                        "ticket_id": {"type": "string", "description": "Номер заявки"},
                        "text": {"type": "string", "description": "Текст обращения (альтернативное поле)"},
                        "previous_results": {"type": "object"}
                    }
                },
                "output_schema": {
                    "type": "object",
                    "properties": {
                        "ticket_id": {"type": "string"},
                        "status": {"type": "string"},
                        "operator": {"type": "string"},
                        "deadline": {"type": "string"}
                    }
                }
            },
            {
                "id": "check_balance",
                "description": "Проверить баланс лицевого счёта",
                "input_schema": {
                    "type": "object",
                    "properties": {
                        "account_id": {"type": "string"},
                        "text": {"type": "string"},
                        "previous_results": {"type": "object"}
                    }
                },
                "output_schema": {
                    "type": "object",
                    "properties": {
                        "account_id": {"type": "string"},
                        "balance": {"type": "number"},
                        "currency": {"type": "string"},
                        "status": {"type": "string"}
                    }
                }
            }
        ],
        "supports": ["a2a/v1", "tasks/send", "mcp/v1"]
    }


@app.get("/.well-known/mcp.json")
async def mcp_discovery():
    return {
        "name": "corporate-agent-mcp",
        "version": "1.0.0",
        "tools": [
            {
                "name": "company_info",
                "description": "Информация о компании, тарифах, услугах"
            },
            {
                "name": "ticket_status",
                "description": "Проверка статуса заявки"
            },
            {
                "name": "check_balance",
                "description": "Проверка баланса счёта"
            }
        ]
    }


# ============ A2A Endpoint ============

@app.post("/tasks/send", response_model=A2ATaskResponse)
async def tasks_send(request: A2ATaskRequest):
    logger.info(f"A2A Task received: skill={request.skill_id}")
    
    try:
        if request.skill_id == "company_info":
            text = request.input.get("text") or request.input.get("query", "")
            result = skill_company_info(text)
            return A2ATaskResponse(
                task_id=str(uuid.uuid4()),
                status="completed",
                output=result
            )
        
        elif request.skill_id == "ticket_status":
            ticket_id = request.input.get("ticket_id", "")
            # Пробуем извлечь номер заявки из текста
            text = request.input.get("text") or request.input.get("query", "")
            if not ticket_id and text:
                import re
                numbers = re.findall(r'\b\d{3,6}\b', text)
                if numbers:
                    ticket_id = numbers[0]
            result = skill_ticket_status(ticket_id)
            return A2ATaskResponse(
                task_id=str(uuid.uuid4()),
                status="completed",
                output=result
            )
        
        elif request.skill_id == "check_balance":
            account_id = request.input.get("account_id", "")
            result = skill_check_balance(account_id)
            return A2ATaskResponse(
                task_id=str(uuid.uuid4()),
                status="completed",
                output=result
            )
        
        else:
            return A2ATaskResponse(
                task_id=str(uuid.uuid4()),
                status="failed",
                error=f"Unknown skill: {request.skill_id}"
            )
    
    except Exception as e:
        logger.error(f"A2A Task failed: {e}")
        return A2ATaskResponse(
            task_id=str(uuid.uuid4()),
            status="failed",
            error=str(e)
        )


if __name__ == "__main__":
    import uvicorn
    port = int(os.getenv("AGENT_PORT", "8080"))
    uvicorn.run(app, host="0.0.0.0", port=port)