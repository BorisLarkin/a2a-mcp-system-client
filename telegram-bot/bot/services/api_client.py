import aiohttp
import asyncio
from typing import Optional, Dict, Any
from bot.config import config
from bot.logger import logger
from bot.models.ticket import TicketCreate, TicketResponse

class APIClient:
    """Клиент для работы с local-proxy API"""
    
    def __init__(self):
        self.base_url = config.api_url
        self.timeout = aiohttp.ClientTimeout(total=config.api_timeout)
        self.session: Optional[aiohttp.ClientSession] = None
        
    async def __aenter__(self):
        self.session = aiohttp.ClientSession(timeout=self.timeout)
        return self
        
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        if self.session:
            await self.session.close()
            
    async def _make_request(
        self, 
        method: str, 
        endpoint: str, 
        **kwargs
    ) -> Dict[str, Any]:
        """Базовый метод для HTTP запросов"""
        url = f"{self.base_url}{endpoint}"
        
        try:
            async with self.session.request(method, url, **kwargs) as response:
                response.raise_for_status()
                return await response.json()
                
        except aiohttp.ClientError as e:
            logger.error(f"API request failed: {e}")
            raise
            
    async def create_ticket(self, ticket_data: TicketCreate) -> TicketResponse:
        """Создать новый тикет"""
        try:
            data = ticket_data.dict()
            
            response = await self._make_request(
                "POST",
                config.api_endpoint,
                json=data,
                headers={"Content-Type": "application/json"}
            )
            
            return TicketResponse(**response)
            
        except Exception as e:
            logger.error(f"Failed to create ticket: {e}")
            
            # Генерируем временный ID для ответа об ошибке
            from uuid import uuid4
            
            return TicketResponse(
                ticket_id=uuid4(),
                status="failed",
                error=str(e)
            )
            
    async def send_message(
        self, 
        chat_id: int, 
        text: str,
        reply_to_message_id: Optional[int] = None
    ) -> bool:
        """Отправить сообщение клиенту (через local-proxy)"""
        try:
            await self._make_request(
                "POST",
                "/api/v1/messages/send",
                json={
                    "chat_id": chat_id,
                    "text": text,
                    "reply_to_message_id": reply_to_message_id
                }
            )
            return True
            
        except Exception as e:
            logger.error(f"Failed to send message to {chat_id}: {e}")
            return False
            
    async def health_check(self) -> bool:
        """Проверка доступности local-proxy"""
        try:
            await self._make_request("GET", "/health")
            return True
        except:
            return False