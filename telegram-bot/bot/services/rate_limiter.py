import time
from typing import Dict, List
from collections import defaultdict
from bot.config import config
from bot.logger import logger

class RateLimiter:
    """Ограничитель запросов по пользователям"""
    
    def __init__(self):
        self.requests: Dict[int, List[float]] = defaultdict(list)
        self.limit = config.rate_limit_per_user
        self.window = 60  # 1 минута в секундах
        
    def is_allowed(self, user_id: int) -> bool:
        """Проверяет, может ли пользователь отправить сообщение"""
        now = time.time()
        
        # Очищаем старые запросы
        user_requests = self.requests[user_id]
        user_requests = [req_time for req_time in user_requests 
                        if now - req_time < self.window]
        self.requests[user_id] = user_requests
        
        # Проверяем лимит
        if len(user_requests) >= self.limit:
            logger.warning(f"Rate limit exceeded for user {user_id}")
            return False
            
        # Добавляем текущий запрос
        user_requests.append(now)
        return True
        
    def reset(self, user_id: int):
        """Сбросить счетчик для пользователя"""
        if user_id in self.requests:
            del self.requests[user_id]