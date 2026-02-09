import re
from typing import Optional

def validate_message_length(text: str, max_length: int) -> bool:
    """Проверка длины сообщения"""
    return len(text) <= max_length

def contains_sensitive_data(text: str) -> bool:
    """Проверка на конфиденциальные данные (базовая)"""
    patterns = [
        r'\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b',  # Номер карты
        r'\b\d{3}[- ]?\d{3}[- ]?\d{3}[- ]?\d{2}\b',  # СНИЛС
        r'\b[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}\b',  # Email
    ]
    
    for pattern in patterns:
        if re.search(pattern, text):
            return True
            
    return False

def sanitize_message(text: str) -> str:
    """Очистка сообщения от потенциально опасных символов"""
    # Удаляем HTML теги
    text = re.sub(r'<[^>]+>', '', text)
    
    # Ограничиваем длину
    text = text[:4000]
    
    return text.strip()