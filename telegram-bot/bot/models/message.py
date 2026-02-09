"""
Модели для работы с сообщениями
"""

from datetime import datetime
from typing import Optional, Dict, Any, List
from enum import Enum
from pydantic import BaseModel, Field, validator
from uuid import UUID, uuid4


class MessageType(str, Enum):
    """Типы сообщений"""
    TEXT = "text"
    PHOTO = "photo"
    DOCUMENT = "document"
    VOICE = "voice"
    VIDEO = "video"
    LOCATION = "location"
    CONTACT = "contact"


class MessageDirection(str, Enum):
    """Направление сообщения"""
    INCOMING = "incoming"  # От клиента к боту
    OUTGOING = "outgoing"  # От бота к клиенту


class MessageStatus(str, Enum):
    """Статус сообщения"""
    PENDING = "pending"      # Ожидает отправки
    SENT = "sent"           # Отправлено успешно
    DELIVERED = "delivered" # Доставлено (если есть подтверждение)
    FAILED = "failed"       # Ошибка отправки
    READ = "read"          # Прочитано (если есть подтверждение)


class MessageBase(BaseModel):
    """Базовая модель сообщения"""
    chat_id: int
    direction: MessageDirection
    message_type: MessageType = MessageType.TEXT
    content: str
    metadata: Dict[str, Any] = Field(default_factory=dict)


class MessageCreate(MessageBase):
    """Модель для создания сообщения"""
    reply_to_message_id: Optional[int] = None
    user_id: Optional[int] = None


class Message(MessageBase):
    """Полная модель сообщения с системными полями"""
    id: UUID = Field(default_factory=uuid4)
    message_id: Optional[int] = None  # ID сообщения в Telegram
    user_id: Optional[int] = None
    reply_to_message_id: Optional[int] = None
    status: MessageStatus = MessageStatus.PENDING
    error: Optional[str] = None
    attempts: int = 0
    max_attempts: int = 3
    created_at: datetime = Field(default_factory=datetime.now)
    updated_at: datetime = Field(default_factory=datetime.now)
    sent_at: Optional[datetime] = None
    delivered_at: Optional[datetime] = None
    read_at: Optional[datetime] = None
    
    @validator('content')
    def validate_content_length(cls, v):
        if len(v) > 4096:
            raise ValueError('Message content too long (max 4096 characters)')
        return v
    
    @validator('metadata')
    def validate_metadata(cls, v):
        # Ограничиваем размер metadata
        import json
        if len(json.dumps(v)) > 10000:
            raise ValueError('Metadata too large')
        return v
    
    def mark_as_sent(self, telegram_message_id: Optional[int] = None):
        """Пометить сообщение как отправленное"""
        self.status = MessageStatus.SENT
        self.message_id = telegram_message_id
        self.sent_at = datetime.now()
        self.updated_at = datetime.now()
    
    def mark_as_failed(self, error: str):
        """Пометить сообщение как неотправленное"""
        self.status = MessageStatus.FAILED
        self.error = error
        self.attempts += 1
        self.updated_at = datetime.now()
    
    def can_retry(self) -> bool:
        """Можно ли повторно отправить сообщение"""
        return (
            self.status == MessageStatus.FAILED and 
            self.attempts < self.max_attempts
        )


class MessageBatch(BaseModel):
    """Пакет сообщений для batch-обработки"""
    messages: List[MessageCreate]
    priority: int = 1  # 1-10, где 10 - наивысший приоритет
    
    @validator('messages')
    def validate_batch_size(cls, v):
        if len(v) > 100:
            raise ValueError('Batch too large (max 100 messages)')
        return v


class MessageStats(BaseModel):
    """Статистика по сообщениям"""
    total_messages: int = 0
    incoming_count: int = 0
    outgoing_count: int = 0
    success_rate: float = 0.0
    avg_response_time_seconds: Optional[float] = None
    by_hour: Dict[int, int] = Field(default_factory=dict)  # Сообщений по часам
    by_day: Dict[str, int] = Field(default_factory=dict)   # Сообщений по дням