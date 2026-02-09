"""
Очередь сообщений для повторных отправок и batch-обработки
"""

import asyncio
import json
from typing import Optional, List, Dict, Any
from datetime import datetime, timedelta
from pathlib import Path
from uuid import UUID
import aiofiles

from bot.logger import logger
from bot.models.message import Message, MessageCreate, MessageStatus
from bot.config import config


class MessageQueue:
    """Асинхронная очередь сообщений с persistence"""
    
    def __init__(self, storage_path: Optional[str] = None):
        self.storage_path = Path(storage_path or "data/queue")
        self.storage_path.mkdir(parents=True, exist_ok=True)
        
        # In-memory очередь
        self.queue: asyncio.PriorityQueue = None
        self.processing_tasks: Dict[str, asyncio.Task] = {}
        
        # Флаг инициализации
        self._initialized = False
        self._load_messages_task = None
    
    async def initialize(self):
        """Инициализация очереди (должна быть вызвана после запуска event loop)"""
        if self._initialized:
            return
            
        self.queue = asyncio.PriorityQueue()
        
        # Загрузка сообщений из storage при старте
        self._load_messages_task = asyncio.create_task(self._load_persisted_messages())
        self._initialized = True
        logger.info("Message queue initialized")
    
    async def add(self, message: Message, priority: int = 1):
        """Добавить сообщение в очередь"""
        if not self._initialized:
            await self.initialize()
        
        try:
            # Сохраняем на диск
            await self._persist_message(message)
            
            # Добавляем в очередь с приоритетом
            # (меньше число = выше приоритет)
            await self.queue.put((priority, datetime.now(), message))
            
            logger.info(f"Message {message.id} added to queue with priority {priority}")
            
            # Запускаем обработку если еще не запущена
            if not self.processing_tasks.get("processor"):
                self.processing_tasks["processor"] = asyncio.create_task(
                    self._process_queue()
                )
                
        except Exception as e:
            logger.error(f"Failed to add message to queue: {e}")
            raise
    
    async def get(self, timeout: Optional[float] = None) -> Optional[Message]:
        """Получить сообщение из очереди"""
        if not self._initialized:
            await self.initialize()
        
        try:
            if timeout:
                item = await asyncio.wait_for(self.queue.get(), timeout)
            else:
                item = await self.queue.get()
            
            _, _, message = item
            return message
            
        except asyncio.TimeoutError:
            return None
        except Exception as e:
            logger.error(f"Failed to get message from queue: {e}")
            return None
    
    async def retry_failed_messages(self):
        """Повторная обработка failed сообщений"""
        if not self._initialized:
            await self.initialize()
        
        failed_messages = await self._get_failed_messages()
        
        for message in failed_messages:
            if message.can_retry():
                logger.info(f"Retrying message {message.id} (attempt {message.attempts + 1})")
                await self.add(message, priority=10)  # Высокий приоритет для retry
    
    async def cleanup_old_messages(self, days: int = 7):
        """Очистка старых сообщений"""
        if not self._initialized:
            await self.initialize()
        
        cutoff_date = datetime.now() - timedelta(days=days)
        old_messages = await self._get_old_messages(cutoff_date)
        
        for message in old_messages:
            await self._delete_message(message.id)
        
        logger.info(f"Cleaned up {len(old_messages)} old messages")
    
    async def get_stats(self) -> Dict[str, Any]:
        """Получить статистику очереди"""
        if not self._initialized:
            await self.initialize()
        
        all_messages = await self._get_all_messages()
        
        stats = {
            "queue_size": self.queue.qsize() if self.queue else 0,
            "total_messages": len(all_messages),
            "pending": len([m for m in all_messages if m.status == MessageStatus.PENDING]),
            "failed": len([m for m in all_messages if m.status == MessageStatus.FAILED]),
            "in_progress": len(self.processing_tasks),
            "storage_size_mb": await self._get_storage_size(),
        }
        
        return stats
    
    async def shutdown(self):
        """Корректное завершение работы очереди"""
        if self._load_messages_task:
            self._load_messages_task.cancel()
            try:
                await self._load_messages_task
            except asyncio.CancelledError:
                pass
        
        # Отменяем все задачи обработки
        for task_name, task in self.processing_tasks.items():
            if task and not task.done():
                task.cancel()
                try:
                    await task
                except asyncio.CancelledError:
                    pass
        
        self._initialized = False
        logger.info("Message queue shutdown completed")
    
    # Private methods
    
    async def _process_queue(self):
        """Фоновая обработка очереди"""
        logger.info("Queue processor started")
        
        try:
            while True:
                message = await self.get(timeout=5.0)
                
                if not message:
                    # Нет сообщений в очереди, небольшая пауза
                    await asyncio.sleep(1)
                    continue
                
                # Обрабатываем сообщение
                task = asyncio.create_task(
                    self._process_message(message),
                    name=f"process_{message.id}"
                )
                self.processing_tasks[str(message.id)] = task
                
                # Ограничиваем concurrent processing
                if len(self.processing_tasks) > 10:
                    await asyncio.sleep(0.1)
                    
        except asyncio.CancelledError:
            logger.info("Queue processor stopped")
        except Exception as e:
            logger.error(f"Queue processor error: {e}")
            # Перезапускаем через некоторое время
            await asyncio.sleep(5)
            self.processing_tasks["processor"] = asyncio.create_task(
                self._process_queue()
            )
    
    async def _process_message(self, message: Message):
        """Обработка одного сообщения"""
        try:
            # Импортируем здесь чтобы избежать циклических импортов
            from bot.services.api_client import APIClient
            
            async with APIClient() as api_client:
                success = await api_client.send_message(
                    chat_id=message.chat_id,
                    text=message.content,
                    reply_to_message_id=message.reply_to_message_id
                )
                
                if success:
                    message.mark_as_sent()
                    logger.info(f"Message {message.id} sent successfully")
                else:
                    message.mark_as_failed("API send failed")
                    logger.warning(f"Message {message.id} failed to send")
                
                # Обновляем на диске
                await self._persist_message(message)
                
        except Exception as e:
            message.mark_as_failed(str(e))
            logger.error(f"Failed to process message {message.id}: {e}")
            await self._persist_message(message)
            
        finally:
            # Удаляем задачу из tracking
            self.processing_tasks.pop(str(message.id), None)
    
    async def _persist_message(self, message: Message):
        """Сохранение сообщения на диск"""
        file_path = self.storage_path / f"{message.id}.json"
        
        try:
            async with aiofiles.open(file_path, 'w', encoding='utf-8') as f:
                data = message.dict()
                data['created_at'] = data['created_at'].isoformat()
                data['updated_at'] = data['updated_at'].isoformat()
                
                if data['sent_at']:
                    data['sent_at'] = data['sent_at'].isoformat()
                if data['delivered_at']:
                    data['delivered_at'] = data['delivered_at'].isoformat()
                if data['read_at']:
                    data['read_at'] = data['read_at'].isoformat()
                
                await f.write(json.dumps(data, ensure_ascii=False, indent=2))
                
        except Exception as e:
            logger.error(f"Failed to persist message {message.id}: {e}")
    
    async def _load_persisted_messages(self):
        """Загрузка сообщений с диска при старте"""
        logger.info("Loading persisted messages...")
        
        try:
            for file_path in self.storage_path.glob("*.json"):
                try:
                    async with aiofiles.open(file_path, 'r', encoding='utf-8') as f:
                        data = json.loads(await f.read())
                        
                        # Конвертируем строки дат обратно в datetime
                        for date_field in ['created_at', 'updated_at', 'sent_at', 
                                         'delivered_at', 'read_at']:
                            if data.get(date_field):
                                data[date_field] = datetime.fromisoformat(data[date_field])
                        
                        message = Message(**data)
                        
                        # Добавляем в очередь если еще не обработано
                        if message.status in [MessageStatus.PENDING, MessageStatus.FAILED]:
                            priority = 5 if message.status == MessageStatus.PENDING else 10
                            await self.queue.put((priority, datetime.now(), message))
                            
                except Exception as e:
                    logger.error(f"Failed to load message from {file_path}: {e}")
            
            logger.info(f"Loaded {self.queue.qsize()} messages from storage")
            
        except Exception as e:
            logger.error(f"Failed to load persisted messages: {e}")
    
    async def _get_all_messages(self) -> List[Message]:
        """Получить все сообщения из storage"""
        messages = []
        
        for file_path in self.storage_path.glob("*.json"):
            try:
                async with aiofiles.open(file_path, 'r', encoding='utf-8') as f:
                    data = json.loads(await f.read())
                    
                    for date_field in ['created_at', 'updated_at', 'sent_at', 
                                     'delivered_at', 'read_at']:
                        if data.get(date_field):
                            data[date_field] = datetime.fromisoformat(data[date_field])
                    
                    messages.append(Message(**data))
                    
            except Exception as e:
                logger.error(f"Failed to read message from {file_path}: {e}")
        
        return messages
    
    async def _get_failed_messages(self) -> List[Message]:
        """Получить все failed сообщения"""
        all_messages = await self._get_all_messages()
        return [m for m in all_messages if m.status == MessageStatus.FAILED]
    
    async def _get_old_messages(self, cutoff_date: datetime) -> List[Message]:
        """Получить старые сообщения"""
        all_messages = await self._get_all_messages()
        return [m for m in all_messages if m.updated_at < cutoff_date]
    
    async def _delete_message(self, message_id: UUID):
        """Удалить сообщение с диска"""
        file_path = self.storage_path / f"{message_id}.json"
        if file_path.exists():
            file_path.unlink()
    
    async def _get_storage_size(self) -> float:
        """Получить размер storage в MB"""
        total_size = 0
        for file_path in self.storage_path.glob("*.json"):
            total_size += file_path.stat().st_size
        
        return total_size / (1024 * 1024)


# Глобальный экземпляр очереди
message_queue = MessageQueue()