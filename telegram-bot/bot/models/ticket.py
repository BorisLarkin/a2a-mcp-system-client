from typing import Optional, Dict, Any
from pydantic import BaseModel, Field
from datetime import datetime
from uuid import UUID, uuid4

class TicketBase(BaseModel):
    """Базовая модель тикета"""
    channel: str = "telegram"
    client_identifier: str  # chat_id
    client_name: Optional[str] = None
    initial_message: str
    metadata: Dict[str, Any] = Field(default_factory=dict)

class TicketCreate(TicketBase):
    """Модель для создания тикета"""
    pass

class TicketResponse(BaseModel):
    """Ответ от local-proxy"""
    ticket_id: UUID
    status: str  # 'new', 'processing', 'completed', 'failed'
    ai_response: Optional[str] = None
    operator_response: Optional[str] = None
    final_response: Optional[str] = None
    ai_confidence: Optional[float] = None
    error: Optional[str] = None
    
class Ticket(TicketBase):
    """Полная модель тикета"""
    id: UUID = Field(default_factory=uuid4)
    status: str = "new"
    ai_response: Optional[str] = None
    operator_response: Optional[str] = None
    final_response: Optional[str] = None
    ai_confidence: Optional[float] = None
    created_at: datetime = Field(default_factory=datetime.now)
    updated_at: datetime = Field(default_factory=datetime.now)